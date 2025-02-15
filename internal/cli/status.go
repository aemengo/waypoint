package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/posener/complete"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"github.com/hashicorp/waypoint/internal/clicontext"
	"github.com/hashicorp/waypoint/internal/clierrors"
	"github.com/hashicorp/waypoint/internal/pkg/flag"
	pb "github.com/hashicorp/waypoint/internal/server/gen"
)

type StatusCommand struct {
	*baseCommand

	flagContextName string
	flagVerbose     bool
	flagJson        bool
	flagAllProjects bool
	filterFlags     filterFlags

	serverCtx *clicontext.Config
}

func (c *StatusCommand) Run(args []string) int {
	flagSet := c.Flags()
	// Initialize. If we fail, we just exit since Init handles the UI.
	if err := c.Init(
		WithArgs(args),
		WithFlags(flagSet),
		WithConfig(true), // optional config loading
	); err != nil {
		return 1
	}

	var ctxName string
	defaultName, err := c.contextStorage.Default()
	if err != nil {
		c.ui.Output(
			"Error getting default context: %s",
			clierrors.Humanize(err),
			terminal.WithErrorStyle(),
		)
		return 1
	}
	ctxName = defaultName

	if ctxName != "" {
		ctxConfig, err := c.contextStorage.Load(ctxName)
		if err != nil {
			c.ui.Output("Error loading context %q: %s", ctxName, err.Error(), terminal.WithErrorStyle())
			return 1
		}
		c.serverCtx = ctxConfig
	} else {
		c.ui.Output(wpNoServerContext, terminal.WithWarningStyle())
	}

	cmdArgs := flagSet.Args()

	if len(cmdArgs) > 1 {
		c.ui.Output("No more than 1 argument required.\n\n"+c.Help(), terminal.WithErrorStyle())
		return 1
	}

	// Determine which view to show based on user input
	var projectTarget, appTarget string
	if len(cmdArgs) >= 1 {
		match := reAppTarget.FindStringSubmatch(cmdArgs[0])

		if match != nil {
			projectTarget = match[1]
			appTarget = match[2]
		} else {
			projectTarget = cmdArgs[0]
		}
	} else if len(cmdArgs) == 0 {
		// If we're in a project dir, load the name. Otherwise we'll
		// show a list of all projects and their status and leave projectTarget
		// blank
		if c.project.Ref() != nil {
			projectTarget = c.project.Ref().Project
		}
	}

	if appTarget == "" && c.flagApp != "" {
		appTarget = c.flagApp
	} else if appTarget != "" && c.flagApp != "" {
		// setting app target and passing the flag app is a collision
		c.ui.Output(wpAppFlagAndTargetIncludedMsg, terminal.WithWarningStyle())
	}

	// Generate a status view
	if projectTarget == "" || c.flagAllProjects {
		// Show high-level status of all projects
		err = c.FormatProjectStatus()
		if err != nil {
			c.ui.Output("CLI failed to build project statuses: "+clierrors.Humanize(err), terminal.WithErrorStyle())
			return 1
		}
	} else if projectTarget != "" && appTarget == "" {
		// Show status of apps inside project
		err = c.FormatProjectAppStatus(projectTarget)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				var serverAddress string
				if c.serverCtx != nil {
					serverAddress = c.serverCtx.Server.Address
				}

				c.ui.Output(wpProjectNotFound, projectTarget, serverAddress, terminal.WithErrorStyle())
			} else {
				c.ui.Output("CLI failed to format project app statuses:"+clierrors.Humanize(err), terminal.WithErrorStyle())
			}
			return 1
		}
	} else if projectTarget != "" && appTarget != "" {
		// Advanced view of a single app status
		err = c.FormatAppStatus(projectTarget, appTarget)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				var serverAddress string
				if c.serverCtx != nil {
					serverAddress = c.serverCtx.Server.Address
				}

				c.ui.Output(wpAppNotFound, appTarget, projectTarget, serverAddress, terminal.WithErrorStyle())
			} else {
				c.ui.Output("CLI failed to format app status:"+clierrors.Humanize(err), terminal.WithErrorStyle())
			}
			return 1
		}
	}

	return 0
}

// FormatProjectAppStatus formats all applications inside a project
func (c *StatusCommand) FormatProjectAppStatus(projectTarget string) error {
	if !c.flagJson && c.serverCtx != nil {
		c.ui.Output(wpStatusProjectMsg, projectTarget, c.serverCtx.Server.Address)
	}

	// Get our API client
	client := c.project.Client()

	resp, err := client.GetProject(c.Ctx, &pb.GetProjectRequest{
		Project: &pb.Ref_Project{
			Project: projectTarget,
		},
	})
	if err != nil {
		return err
	}
	project := resp.Project

	workspace, err := c.getWorkspaceFromProject(resp)
	if err != nil {
		return err
	}

	// Summary
	//   App list

	appHeaders := []string{
		"App", "Workspace", "Deployment Status", "Deployment Checked", "Release Status", "Release Checked",
	}

	appTbl := terminal.NewTable(appHeaders...)

	appFailures := false
	for _, app := range project.Applications {
		// Get the latest deployment
		deploymentsResp, err := client.UI_ListDeployments(c.Ctx, &pb.UI_ListDeploymentsRequest{
			Application: &pb.Ref_Application{
				Application: app.Name,
				Project:     project.Name,
			},
			Workspace: &pb.Ref_Workspace{
				Workspace: workspace,
			},
			Order: &pb.OperationOrder{
				Order: pb.OperationOrder_COMPLETE_TIME,
				Limit: 1,
			},
		})
		if err != nil {
			return err
		}
		var appDeployStatus *pb.StatusReport
		if deploymentsResp.Deployments != nil && len(deploymentsResp.Deployments) > 0 {
			appDeployStatus = deploymentsResp.Deployments[0].LatestStatusReport
		}

		statusReportComplete, statusReportCheckTime, err := c.FormatStatusReportComplete(appDeployStatus)
		if err != nil {
			return err
		}

		if appDeployStatus != nil {
			if appDeployStatus.Health.HealthStatus == "ERROR" ||
				appDeployStatus.Health.HealthStatus == "DOWN" {
				appFailures = true
			}
		}

		// Get the latest release, if there was one
		releasesResp, err := client.UI_ListReleases(c.Ctx, &pb.UI_ListReleasesRequest{
			Application: &pb.Ref_Application{
				Application: app.Name,
				Project:     project.Name,
			},
			Workspace: &pb.Ref_Workspace{
				Workspace: workspace,
			},
			Order: &pb.OperationOrder{
				Order: pb.OperationOrder_COMPLETE_TIME,
				Limit: 1,
			},
		})
		if err != nil {
			return err
		}
		var appReleaseStatus *pb.StatusReport
		if releasesResp.Releases != nil && len(releasesResp.Releases) > 0 {
			appReleaseStatus = releasesResp.Releases[0].LatestStatusReport
		}

		statusReportCompleteRelease, statusReportCheckTimeRelease, err := c.FormatStatusReportComplete(appReleaseStatus)
		if err != nil {
			return err
		}

		if appReleaseStatus != nil {
			if appDeployStatus.Health.HealthStatus == "ERROR" ||
				appDeployStatus.Health.HealthStatus == "DOWN" {
				appFailures = true
			}
		}

		statusColor := ""
		columns := []string{
			app.Name,
			workspace,
			statusReportComplete,
			statusReportCheckTime,
			statusReportCompleteRelease,
			statusReportCheckTimeRelease,
		}

		// Add column data to table
		appTbl.Rich(
			columns,
			[]string{
				statusColor,
			},
		)
	}

	if c.flagJson {
		c.outputJsonProjectAppStatus(appTbl, project)
	} else {
		c.ui.Output("")
		c.ui.Table(appTbl, terminal.WithStyle("Simple"))
		c.ui.Output("")
		c.ui.Output(wpStatusProjectSuccessMsg)
	}

	if appFailures {
		c.ui.Output("")

		c.ui.Output(wpStatusHealthTriageMsg, projectTarget, terminal.WithWarningStyle())
	}

	return nil
}

func (c *StatusCommand) FormatAppStatus(projectTarget string, appTarget string) error {
	if !c.flagJson && c.serverCtx != nil {
		c.ui.Output(wpStatusAppProjectMsg, appTarget, projectTarget, c.serverCtx.Server.Address)
	}

	// Get our API client
	client := c.project.Client()

	projResp, err := client.GetProject(c.Ctx, &pb.GetProjectRequest{
		Project: &pb.Ref_Project{
			Project: projectTarget,
		},
	})
	if err != nil {
		return err
	}
	project := projResp.Project

	workspace, err := c.getWorkspaceFromProject(projResp)
	if err != nil {
		return err
	}

	// App Summary
	//  Summary of single app
	var app *pb.Application
	for _, a := range project.Applications {
		if a.Name == appTarget {
			app = a
			break
		}
	}
	if app == nil {
		return fmt.Errorf("Did not find application %q in project %q", appTarget, projectTarget)
	}

	// Deployment Summary
	//   Deployment List

	// Get the latest deployment
	respDeployList, err := client.UI_ListDeployments(c.Ctx, &pb.UI_ListDeploymentsRequest{
		Application: &pb.Ref_Application{
			Application: app.Name,
			Project:     project.Name,
		},
		Workspace: &pb.Ref_Workspace{
			Workspace: workspace,
		},
		Order: &pb.OperationOrder{
			Order: pb.OperationOrder_COMPLETE_TIME,
			Limit: 1,
		},
	})
	if err != nil {
		return err
	}

	deployHeaders := []string{
		"App Name", "Version", "Workspace", "Platform", "Artifact", "Lifecycle State",
	}

	deployTbl := terminal.NewTable(deployHeaders...)

	resourcesHeaders := []string{
		"Type", "Name", "Platform", "Health", "Time Created",
	}

	resourcesTbl := terminal.NewTable(resourcesHeaders...)

	deployStatusReportComplete := "N/A"
	var deployStatusReportCheckTime string
	appFailures := false
	if len(respDeployList.Deployments) > 0 {
		deploy := respDeployList.Deployments[0].Deployment
		appDeployStatus := respDeployList.Deployments[0].LatestStatusReport
		statusColor := ""

		var details string
		if deploy.Preload != nil && deploy.Preload.Build != nil {
			if deploy.Preload.Artifact != nil {
				artDetails := fmt.Sprintf("id:%d", deploy.Preload.Artifact.Sequence)
				details = artDetails
			}
			if img, ok := deploy.Preload.Build.Labels["common/image-id"]; ok {
				img = shortImg(img)

				details = details + " image:" + img
			}
		}

		columns := []string{
			deploy.Application.Application,
			fmt.Sprintf("v%d", deploy.Sequence),
			deploy.Workspace.Workspace,
			deploy.Component.Name,
			details,
			deploy.Status.State.String(),
		}

		// Add column data to table
		deployTbl.Rich(
			columns,
			[]string{
				statusColor,
			},
		)

		deployStatusReportComplete, deployStatusReportCheckTime, err = c.FormatStatusReportComplete(appDeployStatus)
		if err != nil {
			return err
		}

		// Deployment Resources Summary
		//   Resources List
		if appDeployStatus != nil {
			if appDeployStatus.Health.HealthStatus == "ERROR" ||
				appDeployStatus.Health.HealthStatus == "DOWN" {
				appFailures = true
			}

			for _, dr := range appDeployStatus.Resources {
				var createdTime string
				if dr.CreatedTime != nil {
					t, err := ptypes.Timestamp(dr.CreatedTime)
					if err != nil {
						return err
					}
					createdTime = humanize.Time(t)
				}

				columns := []string{
					dr.Type,
					dr.Name,
					dr.Platform,
					dr.Health.String(),
					createdTime,
				}

				// Add column data to table
				resourcesTbl.Rich(
					columns,
					[]string{
						statusColor,
					},
				)
			}
		}

	} // else show no table

	// Release Summary
	//   Release List

	releasesResp, err := client.UI_ListReleases(c.Ctx, &pb.UI_ListReleasesRequest{
		Application: &pb.Ref_Application{
			Application: app.Name,
			Project:     project.Name,
		},
		Workspace: &pb.Ref_Workspace{
			Workspace: workspace,
		},
		Order: &pb.OperationOrder{
			Order: pb.OperationOrder_COMPLETE_TIME,
			Limit: 1,
		},
	})
	if err != nil {
		return err
	}

	// Same headers as deploy
	releaseTbl := terminal.NewTable(deployHeaders...)
	releaseResourcesTbl := terminal.NewTable(resourcesHeaders...)

	releaseUnimplemented := true
	releaseStatusReportComplete := "N/A"
	var releaseStatusReportCheckTime string
	if releasesResp.Releases != nil {
		release := releasesResp.Releases[0].Release
		releaseUnimplemented = release.Unimplemented

		if !release.Unimplemented {
			appReleaseStatus := releasesResp.Releases[0].LatestStatusReport

			statusColor := ""

			var details string
			if release.Preload.Artifact != nil {
				artDetails := fmt.Sprintf("id:%d", release.Preload.Artifact.Sequence)
				details = artDetails
			}
			if img, ok := release.Preload.Build.Labels["common/image-id"]; ok {
				img = shortImg(img)

				details = details + " image:" + img
			}

			columns := []string{
				release.Application.Application,
				fmt.Sprintf("v%d", release.Sequence),
				release.Workspace.Workspace,
				release.Component.Name,
				details,
				release.Status.State.String(),
			}

			// Add column data to table
			releaseTbl.Rich(
				columns,
				[]string{
					statusColor,
				},
			)

			releaseStatusReportComplete, releaseStatusReportCheckTime, err = c.FormatStatusReportComplete(appReleaseStatus)
			if err != nil {
				return err
			}

			// Release Resources Summary
			//   Resources List
			if appReleaseStatus != nil {
				if appReleaseStatus.Health.HealthStatus == "ERROR" ||
					appReleaseStatus.Health.HealthStatus == "DOWN" {
					appFailures = true
				}

				for _, rr := range appReleaseStatus.Resources {
					var createdTime string
					if rr.CreatedTime != nil {
						t, err := ptypes.Timestamp(rr.CreatedTime)
						if err != nil {
							return err
						}
						createdTime = humanize.Time(t)
					}

					columns := []string{
						rr.Type,
						rr.Name,
						rr.Platform,
						rr.Health.String(),
						createdTime,
					}

					// Add column data to table
					releaseResourcesTbl.Rich(
						columns,
						[]string{
							statusColor,
						},
					)
				}
			}

		}
	} // else show no table

	appHeaders := []string{
		"App", "Workspace", "Deployment Status", "Deployment Checked", "Release Status", "Release Checked",
	}

	appTbl := terminal.NewTable(appHeaders...)

	statusColor := ""
	columns := []string{
		app.Name,
		workspace,
		deployStatusReportComplete,
		deployStatusReportCheckTime,
		releaseStatusReportComplete,
		releaseStatusReportCheckTime,
	}

	// Add column data to table
	appTbl.Rich(
		columns,
		[]string{
			statusColor,
		},
	)

	// TODO(briancain): we don't yet store a list of recent events per app
	// but it would go here if we did.
	// Recent Events
	//   Events List

	if c.flagJson {
		c.outputJsonAppStatus(appTbl, deployTbl, resourcesTbl, releaseTbl, releaseResourcesTbl, project)
	} else {
		c.ui.Output("")
		c.ui.Output("Application Summary")
		c.ui.Table(appTbl, terminal.WithStyle("Simple"))
		c.ui.Output("")
		c.ui.Output("Deployment Summary")
		c.ui.Table(deployTbl, terminal.WithStyle("Simple"))
		c.ui.Output("")
		c.ui.Output("Deployment Resources Summary")
		c.ui.Table(resourcesTbl, terminal.WithStyle("Simple"))
		c.ui.Output("")

		if !releaseUnimplemented {
			c.ui.Output("Release Summary")
			c.ui.Table(releaseTbl, terminal.WithStyle("Simple"))
			c.ui.Output("")
			c.ui.Output("Release Resources Summary")
			c.ui.Table(releaseResourcesTbl, terminal.WithStyle("Simple"))
			c.ui.Output("")
		}

		c.ui.Output(wpStatusAppSuccessMsg)
	}

	if appFailures {
		c.ui.Output("")

		c.ui.Output(wpStatusHealthTriageMsg, projectTarget, terminal.WithWarningStyle())
	}

	return nil
}

// FormatProjectStatus formats all known projects into a table
func (c *StatusCommand) FormatProjectStatus() error {
	if !c.flagJson && c.serverCtx != nil {
		c.ui.Output(wpStatusMsg, c.serverCtx.Server.Address)
	}

	// Get our API client
	client := c.project.Client()

	projectResp, err := client.ListProjects(c.Ctx, &empty.Empty{})
	if err != nil {
		c.ui.Output("Failed to retrieve all projects:"+clierrors.Humanize(err), terminal.WithErrorStyle())
		return err
	}
	projNameList := projectResp.Projects

	headers := []string{
		"Project", "Workspace", "Deployment Statuses", "Release Statuses",
	}

	tbl := terminal.NewTable(headers...)

	for _, projectRef := range projNameList {
		resp, err := client.GetProject(c.Ctx, &pb.GetProjectRequest{
			Project: projectRef,
		})
		if err != nil {
			return err
		}

		workspace, err := c.getWorkspaceFromProject(resp)
		if err != nil {
			return err
		}

		// Get App Statuses
		var appDeployStatusReports []*pb.StatusReport
		var appReleaseStatusReports []*pb.StatusReport
		for _, app := range resp.Project.Applications {
			// Latest Deployment for app
			respDeployList, err := client.UI_ListDeployments(c.Ctx, &pb.UI_ListDeploymentsRequest{
				Application: &pb.Ref_Application{
					Application: app.Name,
					Project:     resp.Project.Name,
				},
				Workspace: &pb.Ref_Workspace{
					Workspace: workspace,
				},
				Order: &pb.OperationOrder{
					Order: pb.OperationOrder_COMPLETE_TIME,
					Limit: 1,
				},
			})
			if err != nil {
				return err
			}

			var appStatusReportDeploy *pb.StatusReport
			if respDeployList.Deployments != nil && len(respDeployList.Deployments) > 0 {
				appStatusReportDeploy = respDeployList.Deployments[0].LatestStatusReport

				if appStatusReportDeploy != nil {
					appDeployStatusReports = append(appDeployStatusReports, appStatusReportDeploy)
				}
			}

			// Latest Release for app
			respReleaseList, err := client.UI_ListReleases(c.Ctx, &pb.UI_ListReleasesRequest{
				Application: &pb.Ref_Application{
					Application: app.Name,
					Project:     resp.Project.Name,
				},
				Workspace: &pb.Ref_Workspace{
					Workspace: workspace,
				},
				Order: &pb.OperationOrder{
					Order: pb.OperationOrder_COMPLETE_TIME,
					Limit: 1,
				},
			})
			if err != nil {
				return err
			}

			var appStatusReportRelease *pb.StatusReport
			if respReleaseList.Releases != nil && len(respReleaseList.Releases) > 0 {
				appStatusReportRelease = respReleaseList.Releases[0].LatestStatusReport

				if appStatusReportRelease != nil {
					appReleaseStatusReports = append(appReleaseStatusReports, appStatusReportRelease)
				}
			}
		}

		deployStatusReportComplete := c.buildAppStatus(appDeployStatusReports)
		releaseStatusReportComplete := c.buildAppStatus(appReleaseStatusReports)

		statusColor := ""
		columns := []string{
			resp.Project.Name,
			workspace,
			deployStatusReportComplete,
			releaseStatusReportComplete,
		}

		// Add column data to table
		tbl.Rich(
			columns,
			[]string{
				statusColor,
			},
		)
	}

	// Render the table
	if c.flagJson {
		c.outputJsonProjectStatus(tbl)
	} else {
		c.ui.Output("")
		c.ui.Table(tbl, terminal.WithStyle("Simple"))
		c.ui.Output("")
		c.ui.Output(wpStatusSuccessMsg)
	}

	return nil
}

func (c *StatusCommand) outputJsonProjectStatus(t *terminal.Table) error {
	output := make(map[string]interface{})

	// Add server context
	serverContext := map[string]interface{}{}

	var serverAddress, serverPlatform string
	if c.serverCtx != nil {
		serverAddress = c.serverCtx.Server.Address
		serverPlatform = c.serverCtx.Server.Platform
	}

	serverContext["Address"] = serverAddress
	serverContext["ServerPlatform"] = serverPlatform

	output["ServerContext"] = serverContext

	projects := c.formatJsonMap(t)
	output["Projects"] = projects

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	c.ui.Output(string(data))

	return nil
}

func (c *StatusCommand) outputJsonProjectAppStatus(
	t *terminal.Table,
	project *pb.Project,
) error {
	output := make(map[string]interface{})

	// Add server context
	serverContext := map[string]interface{}{}

	var serverAddress, serverPlatform string
	if c.serverCtx != nil {
		serverAddress = c.serverCtx.Server.Address
		serverPlatform = c.serverCtx.Server.Platform
	}

	serverContext["Address"] = serverAddress
	serverContext["ServerPlatform"] = serverPlatform

	output["ServerContext"] = serverContext

	// Add project info
	projectInfo := map[string]interface{}{}
	projectInfo["Name"] = project.Name

	output["Project"] = projectInfo

	app := c.formatJsonMap(t)
	output["Applications"] = app

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	c.ui.Output(string(data))

	return nil
}

func (c *StatusCommand) outputJsonAppStatus(
	appTbl *terminal.Table,
	deployTbl *terminal.Table,
	resourcesTbl *terminal.Table,
	releaseTbl *terminal.Table,
	releaseResourcesTbl *terminal.Table,
	project *pb.Project,
) error {
	output := make(map[string]interface{})

	// Add server context
	serverContext := map[string]interface{}{}

	var serverAddress, serverPlatform string
	if c.serverCtx != nil {
		serverAddress = c.serverCtx.Server.Address
		serverPlatform = c.serverCtx.Server.Platform
	}

	serverContext["Address"] = serverAddress
	serverContext["ServerPlatform"] = serverPlatform

	output["ServerContext"] = serverContext

	// Add project info
	projectInfo := map[string]interface{}{}
	projectInfo["Name"] = project.Name

	output["Project"] = projectInfo

	app := c.formatJsonMap(appTbl)
	output["Applications"] = app

	deploySummary := c.formatJsonMap(deployTbl)
	output["DeploymentSummary"] = deploySummary

	deployResourcesSummary := c.formatJsonMap(resourcesTbl)
	output["DeploymentResourcesSummary"] = deployResourcesSummary

	releasesSummary := c.formatJsonMap(releaseTbl)
	output["ReleasesSummary"] = releasesSummary

	releaseResourcesSummary := c.formatJsonMap(releaseResourcesTbl)
	output["ReleasesResourcesSummary"] = releaseResourcesSummary

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	c.ui.Output(string(data))

	return nil
}

// Status Helpers

func (c *StatusCommand) FormatStatusReportComplete(
	statusReport *pb.StatusReport,
) (string, string, error) {
	statusReportComplete := "N/A"

	if statusReport == nil {
		return statusReportComplete, "", nil
	}

	switch statusReport.Health.HealthStatus {
	case "READY":
		statusReportComplete = "✔ READY"
	case "ALIVE":
		statusReportComplete = "✔ ALIVE"
	case "DOWN":
		statusReportComplete = "✖ DOWN"
	case "PARTIAL":
		statusReportComplete = "● PARTIAL"
	case "UNKNOWN":
		statusReportComplete = "? UNKNOWN"
	}

	t, err := ptypes.Timestamp(statusReport.GeneratedTime)
	if err != nil {
		return statusReportComplete, "", err
	}

	return statusReportComplete, humanize.Time(t), nil
}

func (c *StatusCommand) getWorkspaceFromProject(pr *pb.GetProjectResponse) (string, error) {
	var workspace string

	if len(pr.Workspaces) != 0 {
		if c.flagWorkspace != "" {
			for _, ws := range pr.Workspaces {
				if ws.Workspace.Workspace == c.flagWorkspace {
					workspace = ws.Workspace.Workspace
					break
				}
			}

			if workspace == "" {
				return "", fmt.Errorf("Failed to find project in requested workspace %q", c.flagWorkspace)
			}
		} else {
			// No workspace flag specified, try the "first" one
			workspace = pr.Workspaces[0].Workspace.Workspace
		}
	}

	return workspace, nil
}

// buildAppStatus takes a list of Status Reports and builds a string
// that details each apps status in a human readable format.
func (c *StatusCommand) buildAppStatus(reports []*pb.StatusReport) string {
	var ready, alive, down, unknown int

	for _, sr := range reports {
		switch sr.Health.HealthStatus {
		case "DOWN":
			down++
		case "UNKNOWN":
			unknown++
		case "READY":
			ready++
		case "ALIVE":
			alive++
		}
	}

	var result string
	if ready > 0 {
		result = result + fmt.Sprintf("%v READY ", ready)
	}
	if alive > 0 {
		result = result + fmt.Sprintf("%v ALIVE ", alive)
	}
	if down > 0 {
		result = result + fmt.Sprintf("%v DOWN ", down)
	}
	if alive > 0 {
		result = result + fmt.Sprintf("%v UNKNOWN ", unknown)
	}

	if result == "" {
		result = "N/A"
	}

	return result
}

// Takes a terminal Table and formats it into a map of key values to be used
// for formatting a JSON output response
func (c *StatusCommand) formatJsonMap(t *terminal.Table) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, row := range t.Rows {
		c := map[string]interface{}{}

		for j, r := range row {
			// Remove any whitespacess in key
			header := strings.ReplaceAll(t.Headers[j], " ", "")
			c[header] = r.Value
		}
		result = append(result, c)
	}

	return result
}

func (c *StatusCommand) Flags() *flag.Sets {
	return c.flagSet(0, func(set *flag.Sets) {
		f := set.NewSet("Command Options")

		f.BoolVar(&flag.BoolVar{
			Name:    "verbose",
			Aliases: []string{"V"},
			Target:  &c.flagVerbose,
			Usage:   "Display more details.",
		})

		f.BoolVar(&flag.BoolVar{
			Name:   "json",
			Target: &c.flagJson,
			Usage:  "Output the status information as JSON.",
		})

		f.BoolVar(&flag.BoolVar{
			Name:   "all-projects",
			Target: &c.flagAllProjects,
			Usage:  "Output status about every project in a workspace.",
		})
	})
}

func (c *StatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *StatusCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *StatusCommand) Synopsis() string {
	return "List statuses."
}

func (c *StatusCommand) Help() string {
	return formatHelp(`
Usage: waypoint status [options] [project]

  View the current status of projects and applications managed by Waypoint.

` + c.Flags().Help())
}

var (
	// Success or info messages

	wpStatusSuccessMsg = strings.TrimSpace(`
The projects listed above represent their current state known
in the Waypoint server. For more information about a project’s applications and
their current state, run ‘waypoint status PROJECT-NAME’.
`)

	wpStatusProjectSuccessMsg = strings.TrimSpace(`
The project and its apps listed above represents its current state known
in the Waypoint server. For more information about a project’s applications and
their current state, run ‘waypoint status -app=APP-NAME PROJECT-NAME’.
`)

	wpStatusAppSuccessMsg = strings.TrimSpace(`
The application and its declared resources listed above represents its current state known
in the Waypoint server.
`)

	wpStatusMsg = "Current project statuses in server context %q"

	wpStatusProjectMsg = "Current status for project %q in server context %q."

	wpStatusAppProjectMsg = strings.TrimSpace(`
Current status for application % q in project %q in server context %q.
`)

	// Failure messages

	wpNoServerContext = strings.TrimSpace(`
No default server context set for the Waypoint CLI. To set a default, use
'waypoint context use <context-name>'. To see a full list of known contexts,
run 'waypoint context list'. If Waypoint is running in local mode, this is expected.
`)

	wpStatusHealthTriageMsg = strings.TrimSpace(`
To see more information about the failing application, please check out the application logs:

waypoint logs -app=APP-NAME

The projects listed above represent their current state known
in Waypoint server. For more information about an application defined in the
project %[1]q can be viewed by running the command:

waypoint status -app=APP-NAME %[1]s
`)

	wpProjectNotFound = strings.TrimSpace(`
No project name %q was found for the server context %q. To see a list of
currently configured projects, run “waypoint project list”.

If you want more information for a specific application, use the '-app' flag
with “waypoint status -app=APP-NAME PROJECT-NAME”.
`)

	wpAppFlagAndTargetIncludedMsg = strings.TrimSpace(`
The 'app' flag was included, but an application was also requested as an argument.
The app flag will be ignored.
`)

	wpAppNotFound = strings.TrimSpace(`
No application named %q was found in project %q for the server context %q. To see a
list of currently configured projects, run “waypoint project list”.
`)
)
