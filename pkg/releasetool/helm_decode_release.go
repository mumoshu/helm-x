package releasetool

// Copied and adopted with some modification from k8s.io/helm/pkg/storage/driver/util.go with love

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"

	"github.com/golang/protobuf/proto"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
)

var b64 = base64.StdEncoding

var magicGzip = []byte{0x1f, 0x8b, 0x08}

// encodeRelease encodes a release returning a base64 encoded
// gzipped binary protobuf encoding representation, or error.
func encodeRelease(rls *rspb.Release) (string, error) {
	b, err := proto.Marshal(rls)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err = w.Write(b); err != nil {
		return "", err
	}
	w.Close()

	return b64.EncodeToString(buf.Bytes()), nil
}
