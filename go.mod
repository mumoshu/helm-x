module github.com/mumoshu/helm-x

go 1.12

require (
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.20.0+incompatible // indirect
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/coreos/etcd v3.3.27+incompatible // indirect
	github.com/coreos/pkg v0.0.0-20230327231512-ba87abf18a23 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v20.10.24+incompatible // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20221015165544-a0805db90819 // indirect
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/go-openapi/spec v0.19.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.1
	github.com/variantdev/chartify v0.3.2
	github.com/xlab/handysort v0.0.0-20150421192137-fb3537ed64a1 // indirect
	google.golang.org/grpc v1.54.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.0.0-20190515023547-db5a9d1c40eb
	k8s.io/apiextensions-apiserver v0.0.0-20190515024537-2fd0e9006049 // indirect
	k8s.io/apimachinery v0.0.0-20190515023456-b74e4c97951f
	k8s.io/apiserver v0.0.0-20190515064100-fc28ef5782df // indirect
	k8s.io/cli-runtime v0.0.0-20190515024640-178667528169 // indirect
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20190510232812-a01b7d5d6c22 // indirect
	k8s.io/kubernetes v1.13.1
	k8s.io/utils v0.0.0-20190506122338-8fab8cb257d5 // indirect
	sigs.k8s.io/yaml v1.1.0
	vbom.ml/util v0.0.0-20180919145318-efcd4e0f9787 // indirect
)

// blackfriday needed to avoid:
//		# k8s.io/kubernetes/pkg/kubectl/util/templates
//		../go/pkg/mod/k8s.io/kubernetes@v1.13.1/pkg/kubectl/util/templates/markdown.go:30:5: cannot use &ASCIIRenderer literal (type *ASCIIRenderer) as type blackfriday.Renderer in assignment:
//		*ASCIIRenderer does not implement blackfriday.Renderer (missing RenderFooter method)
//		../go/pkg/mod/k8s.io/kubernetes@v1.13.1/pkg/kubectl/util/templates/markdown.go:64:11: undefined: blackfriday.LIST_ITEM_BEGINNING_OF_LIST
//		../go/pkg/mod/k8s.io/kubernetes@v1.13.1/pkg/kubectl/util/templates/markdown.go:71:11: undefined: blackfriday.LIST_TYPE_ORDERED
//		../go/pkg/mod/k8s.io/kubernetes@v1.13.1/pkg/kubectl/util/templates/normalizers.go:73:35: too many arguments to conversion to blackfriday.Markdown: blackfriday.Markdown(bytes, composite literal, 0)

replace github.com/russross/blackfriday => github.com/russross/blackfriday v1.5.2

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20181213151034-8d9ed539ba31

replace k8s.io/api => k8s.io/api v0.0.0-20181213150558-05914d821849

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20181213153335-0fe22c71c476

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20181213151703-3ccfe8365421

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20181213153952-835b10687cb6
