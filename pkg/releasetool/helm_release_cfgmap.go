package releasetool

// Copied from https://github.com/helm/helm/blob/90f50a11db5e81be0edd179b60a50adb9fcf3942/pkg/storage/driver/cfgmaps.go#L232 with love

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"strconv"
)

func newConfigMapsObject(key string, rls *rspb.Release, lbs labels) (*v1.ConfigMap, error) {
	const owner = "TILLER"

	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	if lbs == nil {
		lbs.init()
	}

	// c labels
	lbs.set("NAME", rls.Name)
	lbs.set("OWNER", owner)
	lbs.set("STATUS", rspb.Status_Code_name[int32(rls.Info.Status.Code)])
	lbs.set("VERSION", strconv.Itoa(int(rls.Version)))

	// create and return configmap object
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   key,
			Labels: lbs.toMap(),
		},
		Data: map[string]string{"release": s},
	}, nil
}
