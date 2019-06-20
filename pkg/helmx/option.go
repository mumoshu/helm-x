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

var _ AdoptOption = &tillerNamespace{}
var _ DiffOption = &tillerNamespace{}

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

var _ AdoptOption = &namespace{}
var _ DiffOption = &namespace{}

type storage struct {
	storage string
}

func (s *storage) SetAdoptOption(o *AdoptOpts) error {
	if o.ClientOpts == nil {
		o.ClientOpts = &ClientOpts{}
	}
	o.ClientOpts.TillerStorageBackend = s.storage
	return nil
}

func TillerStorageBackend(s string) *storage {
	return &storage{storage: s}
}

var _ AdoptOption = &storage{}
