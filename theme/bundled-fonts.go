package theme

import (
	_ "embed"

	"github.com/belluikit/bellui"
)

//go:embed font/Roboto-Regular.ttf
var robotoRegular []byte

var regular = &bellui.StaticResource{
	StaticName:    "Roboto-Regular.ttf",
	StaticContent: robotoRegular,
}

//go:embed font/Roboto-Bold.ttf
var robotoBold []byte

var bold = &bellui.StaticResource{
	StaticName:    "Roboto-Bold.ttf",
	StaticContent: robotoBold,
}

//go:embed font/Roboto-Italic.ttf
var robotoItalic []byte

var italic = &bellui.StaticResource{
	StaticName:    "Roboto-Italic.ttf",
	StaticContent: robotoItalic,
}

//go:embed font/Roboto-BoldItalic.ttf
var robotoBoldItalic []byte

var bolditalic = &bellui.StaticResource{
	StaticName:    "Roboto-BoldItalic.ttf",
	StaticContent: robotoBoldItalic,
}

//go:embed font/Roboto-Medium.ttf
var robotoMedium []byte

var medium = &bellui.StaticResource{
	StaticName:    "Roboto-Medium.ttf",
	StaticContent: robotoMedium,
}

//go:embed font/Roboto-MediumItalic.ttf
var robotoMediumItalic []byte

var mediumItalic = &bellui.StaticResource{
	StaticName:    "Roboto-MediumItalic.ttf",
	StaticContent: robotoMediumItalic,
}

//go:embed font/OfficeCodePro-Regular.otf
var officeCodeProRegular []byte

var monospace = &bellui.StaticResource{
	StaticName:    "OfficeCodePro-Regular.otf",
	StaticContent: officeCodeProRegular,
}

//go:embed font/OfficeCodePro-Italic.otf
var officeCodeProItalic []byte

var monoItalicRes = &bellui.StaticResource{
	StaticName:    "OfficeCodePro-Italic.otf",
	StaticContent: officeCodeProItalic,
}

//go:embed font/OfficeCodePro-Bold.otf
var officeCodeProBold []byte

var monoBoldRes = &bellui.StaticResource{
	StaticName:    "OfficeCodePro-Bold.otf",
	StaticContent: officeCodeProBold,
}

//go:embed font/OfficeCodePro-BoldItalic.otf
var officeCodeProBoldItalic []byte

var monoBoldItalicRes = &bellui.StaticResource{
	StaticName:    "OfficeCodePro-BoldItalic.otf",
	StaticContent: officeCodeProBoldItalic,
}

//go:embed font/InterSymbols-Regular.ttf
var interSymbolsRegular []byte

var symbol = &bellui.StaticResource{
	StaticName:    "InterSymbols-Regular.ttf",
	StaticContent: interSymbolsRegular,
}
