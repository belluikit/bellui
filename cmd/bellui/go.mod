module github.com/belluikit/bellui/cmd/bellui

go 1.25.0

require (
	github.com/belluikit/gocef-libs v0.2.0
	github.com/fogleman/gg v1.3.0
	github.com/go-ole/go-ole v1.3.0
	github.com/jackmordaunt/icns/v2 v2.2.6
	github.com/josephspurrier/goversioninfo v1.4.0
	github.com/klauspost/compress v1.19.1
	github.com/lucor/goinfo v0.9.0
	github.com/natefinch/atomic v1.0.1
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/stretchr/testify v1.11.1
	github.com/trendvidia/protowire-go v1.2.2
	github.com/urfave/cli/v2 v2.27.1
	golang.org/x/mod v0.35.0
	golang.org/x/sys v0.47.0
	golang.org/x/tools v0.44.0
	golang.org/x/tools/go/vcs v0.1.0-deprecated
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/belluikit/bellui v1.2.0
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	golang.org/x/image v0.44.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v1.26.1 // Contains only retraction as v1.6.1 was ignored
	v1.26.0 // Published accidentally.
)
