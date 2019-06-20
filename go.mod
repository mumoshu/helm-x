module github.com/mumoshu/helm-x

go 1.12

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.20.0+incompatible // indirect
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/go-openapi/spec v0.19.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0 // indirect
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/kubernetes/cli-runtime v0.0.0-20190516231937-17bc0b7fcef5 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/otiai10/copy v0.0.0-20180813032824-7e9a647135a1
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.1
	golang.org/x/crypto v0.0.0-20190513172903-22d7a77e9e5f // indirect
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gopkg.in/yaml.v3 v3.0.0-20190409140830-cdc409dda467
	k8s.io/api v0.0.0-20190515023547-db5a9d1c40eb
	k8s.io/apiextensions-apiserver v0.0.0-20190515024537-2fd0e9006049 // indirect
	k8s.io/apimachinery v0.0.0-20190515023456-b74e4c97951f
	k8s.io/apiserver v0.0.0-20190515064100-fc28ef5782df // indirect
	k8s.io/cli-runtime v0.0.0-20190515024640-178667528169
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/klog v0.3.0
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
