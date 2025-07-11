// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.30.2
// source: example.proto

package examplepb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Request struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       string                 `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Request) Reset() {
	*x = Request{}
	mi := &file_example_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Request) ProtoMessage() {}

func (x *Request) ProtoReflect() protoreflect.Message {
	mi := &file_example_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Request.ProtoReflect.Descriptor instead.
func (*Request) Descriptor() ([]byte, []int) {
	return file_example_proto_rawDescGZIP(), []int{0}
}

func (x *Request) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type Response struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       string                 `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Response) Reset() {
	*x = Response{}
	mi := &file_example_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Response) ProtoMessage() {}

func (x *Response) ProtoReflect() protoreflect.Message {
	mi := &file_example_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Response.ProtoReflect.Descriptor instead.
func (*Response) Descriptor() ([]byte, []int) {
	return file_example_proto_rawDescGZIP(), []int{1}
}

func (x *Response) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_example_proto protoreflect.FileDescriptor

const file_example_proto_rawDesc = "" +
	"\n" +
	"\rexample.proto\x12\aexample\"#\n" +
	"\aRequest\x12\x18\n" +
	"\amessage\x18\x01 \x01(\tR\amessage\"$\n" +
	"\bResponse\x12\x18\n" +
	"\amessage\x18\x01 \x01(\tR\amessage2\x85\x02\n" +
	"\x0eExampleService\x120\n" +
	"\tUnaryCall\x12\x10.example.Request\x1a\x11.example.Response\x12<\n" +
	"\x13ServerStreamingCall\x12\x10.example.Request\x1a\x11.example.Response0\x01\x12<\n" +
	"\x13ClientStreamingCall\x12\x10.example.Request\x1a\x11.example.Response(\x01\x12E\n" +
	"\x1aBidirectionalStreamingCall\x12\x10.example.Request\x1a\x11.example.Response(\x010\x01B\x03Z\x01/b\x06proto3"

var (
	file_example_proto_rawDescOnce sync.Once
	file_example_proto_rawDescData []byte
)

func file_example_proto_rawDescGZIP() []byte {
	file_example_proto_rawDescOnce.Do(func() {
		file_example_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_example_proto_rawDesc), len(file_example_proto_rawDesc)))
	})
	return file_example_proto_rawDescData
}

var file_example_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_example_proto_goTypes = []any{
	(*Request)(nil),  // 0: example.Request
	(*Response)(nil), // 1: example.Response
}
var file_example_proto_depIdxs = []int32{
	0, // 0: example.ExampleService.UnaryCall:input_type -> example.Request
	0, // 1: example.ExampleService.ServerStreamingCall:input_type -> example.Request
	0, // 2: example.ExampleService.ClientStreamingCall:input_type -> example.Request
	0, // 3: example.ExampleService.BidirectionalStreamingCall:input_type -> example.Request
	1, // 4: example.ExampleService.UnaryCall:output_type -> example.Response
	1, // 5: example.ExampleService.ServerStreamingCall:output_type -> example.Response
	1, // 6: example.ExampleService.ClientStreamingCall:output_type -> example.Response
	1, // 7: example.ExampleService.BidirectionalStreamingCall:output_type -> example.Response
	4, // [4:8] is the sub-list for method output_type
	0, // [0:4] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_example_proto_init() }
func file_example_proto_init() {
	if File_example_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_example_proto_rawDesc), len(file_example_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_example_proto_goTypes,
		DependencyIndexes: file_example_proto_depIdxs,
		MessageInfos:      file_example_proto_msgTypes,
	}.Build()
	File_example_proto = out.File
	file_example_proto_goTypes = nil
	file_example_proto_depIdxs = nil
}
