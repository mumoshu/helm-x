package helmx

type tillerNamespace struct {
	tillerNs string
}

func (n *tillerNamespace) SetAdoptOption(o *AdoptOpts) error {
	o.TillerNamespace = n.tillerNs
	return nil
}

func (n *tillerNamespace) SetDiffOption(o *DiffOpts) error {
	o.TillerNamespace = n.tillerNs
	return nil
}

func TillerNamespace(tillerNs string) *tillerNamespace {
	return &tillerNamespace{tillerNs: tillerNs}
}

var _ adoptOption = &tillerNamespace{}
var _ diffOption = &tillerNamespace{}

type namespace struct {
	ns string
}

func (n *namespace) SetAdoptOption(o *AdoptOpts) error {
	o.Namespace = n.ns
	return nil
}

func (n *namespace) SetDiffOption(o *DiffOpts) error {
	o.Namespace = n.ns
	return nil
}

func Namespace(ns string) *namespace {
	return &namespace{ns: ns}
}

var _ adoptOption = &namespace{}
var _ diffOption = &namespace{}
