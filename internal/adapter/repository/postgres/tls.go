package postgres

import (
	"crypto/x509"
	_ "embed"
	"errors"
	"sync"
)

// rdsBundlePEM is Amazon's global RDS certificate bundle, committed to this
// repository and compiled into the binary.
//
// It is embedded rather than shipped as a file because a release here is a
// directory that deploy.sh swaps by symlink and prunes on a schedule: a cert
// file in that directory is one more thing that can go missing, go stale
// relative to the binary, or fail to upload. A root bundle the binary carries
// cannot desynchronise from the code that trusts it.
//
// Refreshing it is therefore a code change, not a config change:
//
//	curl -fsSLo internal/adapter/repository/postgres/rds-global-bundle.pem \
//	  https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
//
//go:embed rds-global-bundle.pem
var rdsBundlePEM []byte

// rdsRoots parses the embedded bundle once.
//
// A parse failure is fatal rather than a fallback to the system pool. The
// system pool does not contain the Amazon RDS CAs, so "fall back" would mean
// every connection failing later with a certificate error far from its cause --
// or, worse, silently succeeding if someone had also relaxed sslmode.
var rdsRoots = sync.OnceValues(func() (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rdsBundlePEM) {
		return nil, errors.New("embedded RDS CA bundle contains no certificates")
	}
	return pool, nil
})
