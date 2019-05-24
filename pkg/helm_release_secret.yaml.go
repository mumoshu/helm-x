package x

// Copied from https://github.com/helm/helm/blob/90f50a11db5e81be0edd179b60a50adb9fcf3942/pkg/storage/driver/secrets.go#L232 with love

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func newSecretsObject(key string, rls *rspb.Release, lbs labels) (*v1.Secret, error) {
	const owner = "TILLER"

	// encode the release
	s, err := encodeRelease(rls)
	if err != nil {
		return nil, err
	}

	if lbs == nil {
		lbs.init()
	}

	// apply labels
	lbs.set("NAME", rls.Name)
	lbs.set("OWNER", owner)
	lbs.set("STATUS", rspb.Status_Code_name[int32(rls.Info.Status.Code)])
	lbs.set("VERSION", strconv.Itoa(int(rls.Version)))

	// create and return secret object
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   key,
			Labels: lbs.toMap(),
		},
		Data: map[string][]byte{"release": []byte(s)},
	}, nil
}
