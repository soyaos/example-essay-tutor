module github.com/soyaos/example-essay-tutor/e2e

go 1.23.0

// SoyaOS uses a Go workspace with independently versioned modules. Local
// replaces keep this E2E suite pinned to the sibling source checkout:
//
//	~/workspace/soyaos/soyaos              <- core monorepo
//	~/workspace/soyaos/example-essay-tutor <- this repo

require (
	github.com/chromedp/chromedp v0.11.2
	github.com/soyaos/soyaos/pkg/artifact v0.1.0-alpha.1
	github.com/soyaos/soyaos/pkg/auth v0.1.0-alpha.2
	github.com/soyaos/soyaos/pkg/kernel v0.1.0-alpha.2
	github.com/soyaos/soyaos/pkg/openaicompat v0.1.0-alpha.1
	github.com/soyaos/soyaos/pkg/soyapack v0.1.0-alpha.2
)

replace github.com/soyaos/soyaos/pkg/artifact => ../../soyaos/pkg/artifact

replace github.com/soyaos/soyaos/pkg/auth => ../../soyaos/pkg/auth

replace github.com/soyaos/soyaos/pkg/kernel => ../../soyaos/pkg/kernel

replace github.com/soyaos/soyaos/pkg/openaicompat => ../../soyaos/pkg/openaicompat

replace github.com/soyaos/soyaos/pkg/soyapack => ../../soyaos/pkg/soyapack

require (
	github.com/chromedp/cdproto v0.0.0-20241022234722-4d5d5faf59fb // indirect
	github.com/chromedp/sysutil v1.1.0 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/richardlehane/mscfb v1.0.6 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/soyaos/soyaos/pkg/llmcall v0.1.0-alpha.2 // indirect
	github.com/soyaos/soyaos/pkg/state v0.1.0-alpha.2 // indirect
	github.com/soyaos/soyaos/pkg/store v0.1.0-alpha.2 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/excelize/v2 v2.9.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
