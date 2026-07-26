package app

// these internal variables are set by the bellui build command so that the "manifest.pxf" data is readable at runtime.
var (
	MetaIcon    = "" // this will contain base64 encoded icon bytes
	MetaID      = "com.example"
	MetaName    = "Bellui App"
	MetaVersion = "1.0.0"
	MetaBuild   = "1"
)
