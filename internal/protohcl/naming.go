package protohcl

import "google.golang.org/protobuf/reflect/protoreflect"

type FieldNamer interface {
	NameField(protoreflect.FieldDescriptor) string
	GetField(protoreflect.FieldDescriptors, string) protoreflect.FieldDescriptor
}

type textFieldNamer struct{}

func (textFieldNamer) NameField(fd protoreflect.FieldDescriptor) string {
	return fd.TextName()
}

func (textFieldNamer) GetField(fds protoreflect.FieldDescriptors, name string) protoreflect.FieldDescriptor {
	return fds.ByTextName(name)
}
