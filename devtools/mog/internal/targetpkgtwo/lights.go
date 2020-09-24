package targetpkgtwo

type Lamp struct {
	Brand   string
	Sockets uint8
}

type Flood struct {
	StructIsAlsoAField bool
}

type StructIsAlsoAField struct {
	ID Identifier
}

type Identifier struct {
	Name      string
	Namespace string
}
