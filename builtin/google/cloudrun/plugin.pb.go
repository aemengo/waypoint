// Code generated by protoc-gen-go. DO NOT EDIT.
// source: plugin.proto

package cloudrun

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Deployment struct {
	Url                  string               `protobuf:"bytes,1,opt,name=url,proto3" json:"url,omitempty"`
	Resource             *Deployment_Resource `protobuf:"bytes,2,opt,name=resource,proto3" json:"resource,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *Deployment) Reset()         { *m = Deployment{} }
func (m *Deployment) String() string { return proto.CompactTextString(m) }
func (*Deployment) ProtoMessage()    {}
func (*Deployment) Descriptor() ([]byte, []int) {
	return fileDescriptor_plugin_739d4b0ac8679631, []int{0}
}
func (m *Deployment) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Deployment.Unmarshal(m, b)
}
func (m *Deployment) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Deployment.Marshal(b, m, deterministic)
}
func (dst *Deployment) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Deployment.Merge(dst, src)
}
func (m *Deployment) XXX_Size() int {
	return xxx_messageInfo_Deployment.Size(m)
}
func (m *Deployment) XXX_DiscardUnknown() {
	xxx_messageInfo_Deployment.DiscardUnknown(m)
}

var xxx_messageInfo_Deployment proto.InternalMessageInfo

func (m *Deployment) GetUrl() string {
	if m != nil {
		return m.Url
	}
	return ""
}

func (m *Deployment) GetResource() *Deployment_Resource {
	if m != nil {
		return m.Resource
	}
	return nil
}

type Deployment_Resource struct {
	Location             string   `protobuf:"bytes,1,opt,name=location,proto3" json:"location,omitempty"`
	Project              string   `protobuf:"bytes,2,opt,name=project,proto3" json:"project,omitempty"`
	Name                 string   `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Deployment_Resource) Reset()         { *m = Deployment_Resource{} }
func (m *Deployment_Resource) String() string { return proto.CompactTextString(m) }
func (*Deployment_Resource) ProtoMessage()    {}
func (*Deployment_Resource) Descriptor() ([]byte, []int) {
	return fileDescriptor_plugin_739d4b0ac8679631, []int{0, 0}
}
func (m *Deployment_Resource) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Deployment_Resource.Unmarshal(m, b)
}
func (m *Deployment_Resource) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Deployment_Resource.Marshal(b, m, deterministic)
}
func (dst *Deployment_Resource) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Deployment_Resource.Merge(dst, src)
}
func (m *Deployment_Resource) XXX_Size() int {
	return xxx_messageInfo_Deployment_Resource.Size(m)
}
func (m *Deployment_Resource) XXX_DiscardUnknown() {
	xxx_messageInfo_Deployment_Resource.DiscardUnknown(m)
}

var xxx_messageInfo_Deployment_Resource proto.InternalMessageInfo

func (m *Deployment_Resource) GetLocation() string {
	if m != nil {
		return m.Location
	}
	return ""
}

func (m *Deployment_Resource) GetProject() string {
	if m != nil {
		return m.Project
	}
	return ""
}

func (m *Deployment_Resource) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func init() {
	proto.RegisterType((*Deployment)(nil), "google.cloudrun.Deployment")
	proto.RegisterType((*Deployment_Resource)(nil), "google.cloudrun.Deployment.Resource")
}

func init() { proto.RegisterFile("plugin.proto", fileDescriptor_plugin_739d4b0ac8679631) }

var fileDescriptor_plugin_739d4b0ac8679631 = []byte{
	// 183 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0xc8, 0x29, 0x4d,
	0xcf, 0xcc, 0xd3, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x4f, 0xcf, 0xcf, 0x4f, 0xcf, 0x49,
	0xd5, 0x4b, 0xce, 0xc9, 0x2f, 0x4d, 0x29, 0x2a, 0xcd, 0x53, 0xda, 0xc6, 0xc8, 0xc5, 0xe5, 0x92,
	0x5a, 0x90, 0x93, 0x5f, 0x99, 0x9b, 0x9a, 0x57, 0x22, 0x24, 0xc0, 0xc5, 0x5c, 0x5a, 0x94, 0x23,
	0xc1, 0xa8, 0xc0, 0xa8, 0xc1, 0x19, 0x04, 0x62, 0x0a, 0x39, 0x70, 0x71, 0x14, 0xa5, 0x16, 0xe7,
	0x97, 0x16, 0x25, 0xa7, 0x4a, 0x30, 0x29, 0x30, 0x6a, 0x70, 0x1b, 0xa9, 0xe8, 0xa1, 0x19, 0xa2,
	0x87, 0x30, 0x40, 0x2f, 0x08, 0xaa, 0x36, 0x08, 0xae, 0x4b, 0x2a, 0x84, 0x8b, 0x03, 0x26, 0x2a,
	0x24, 0xc5, 0xc5, 0x91, 0x93, 0x9f, 0x9c, 0x58, 0x92, 0x99, 0x9f, 0x07, 0xb5, 0x04, 0xce, 0x17,
	0x92, 0xe0, 0x62, 0x2f, 0x28, 0xca, 0xcf, 0x4a, 0x4d, 0x2e, 0x01, 0x5b, 0xc4, 0x19, 0x04, 0xe3,
	0x0a, 0x09, 0x71, 0xb1, 0xe4, 0x25, 0xe6, 0xa6, 0x4a, 0x30, 0x83, 0x85, 0xc1, 0x6c, 0x27, 0xae,
	0x28, 0x0e, 0x98, 0xfd, 0x49, 0x6c, 0x60, 0xcf, 0x19, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0x06,
	0x4a, 0xfc, 0x45, 0xec, 0x00, 0x00, 0x00,
}
