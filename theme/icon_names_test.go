package theme

import (
	"io"
	"log"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage/repository"
)

// TestIconNamesFollowPlayer pins every icon name Player apps use for the
// same icon (#1307, belluikit/player#726): snake_case, Material's names.
// The icons Player leaves unnamed (widget parts) are not listed.
func TestIconNamesFollowPlayer(t *testing.T) {
	want := map[bellui.ThemeIconName]string{
		IconNameContentAdd:        "add",
		IconNameContentRemove:     "remove",
		IconNameDelete:            "delete",
		IconNameDocumentCreate:    "edit",
		IconNameDocumentSave:      "save",
		IconNameSearch:            "search",
		IconNameSettings:          "settings",
		IconNameHome:              "home",
		IconNameAccount:           "account",
		IconNameInfo:              "info",
		IconNameQuestion:          "help",
		IconNameConfirm:           "check",
		IconNameCancel:            "cancel",
		IconNameNavigateBack:      "back",
		IconNameNavigateNext:      "forward",
		IconNameMenu:              "menu",
		IconNameMoreVertical:      "more",
		IconNameMoreHorizontal:    "more_horiz",
		IconNameMailCompose:       "mail",
		IconNameDownload:          "download",
		IconNameUpload:            "upload",
		IconNameViewRefresh:       "refresh",
		IconNameVisibility:        "visibility",
		IconNameVisibilityOff:     "visibility_off",
		IconNameMediaPlay:         "play",
		IconNameMediaPause:        "pause",
		IconNameMediaStop:         "stop",
		IconNameMediaSkipNext:     "skip_next",
		IconNameMediaSkipPrevious: "skip_previous",
		IconNameMediaFastForward:  "fast_forward",
		IconNameMediaFastRewind:   "fast_rewind",
		IconNameMediaReplay:       "replay",
		IconNameMediaMusic:        "music",
		IconNameHistory:           "history",
		IconNameList:              "list",
		IconNameVolumeUp:          "volume_up",
		IconNameVolumeDown:        "volume_down",
		IconNameVolumeMute:        "volume_mute",
		IconNameFavorite:          "favorite",
		IconNameFavoriteBorder:    "favorite_border",
		IconNameBookmark:          "bookmark",
		IconNameBookmarkBorder:    "bookmark_border",
		IconNamePlace:             "place",
		IconNamePeople:            "people",
		IconNameChild:             "child",
		IconNameContentCopy:       "content_copy",
		IconNameClosedCaption:     "closed_caption",
		IconNameClosedCaptionOff:  "closed_caption_off",
		IconNameNotifications:     "notifications",
		IconNameShoppingCart:      "shopping_cart",
		IconNameAddShoppingCart:   "add_shopping_cart",
		IconNameStorefront:        "storefront",
		IconNameShoppingBag:       "shopping_bag",
		IconNameWorkspacePremium:  "workspace_premium",
		IconNameDiamond:           "diamond",
		IconNameAutorenew:         "autorenew",
		IconNameCardMembership:    "card_membership",
		IconNameCreditCard:        "credit_card",
		IconNameReceiptLong:       "receipt_long",
		IconNameLock:              "lock",
		IconNameContentCut:        "cut",
		IconNameContentPaste:      "paste",
		IconNameContentUndo:       "undo",
		IconNameContentRedo:       "redo",
		IconNameContentClear:      "clear",
		IconNameSearchReplace:     "find_replace",
		IconNameFile:              "file",
		IconNameFileText:          "file_text",
		IconNameFileImage:         "file_image",
		IconNameFileAudio:         "file_audio",
		IconNameFileVideo:         "file_video",
		IconNameFileApplication:   "file_app",
		IconNameFolder:            "folder",
		IconNameFolderOpen:        "folder_open",
		IconNameFolderNew:         "create_new_folder",
		IconNameDocument:          "document",
		IconNameDocumentPrint:     "print",
		IconNameStorage:           "storage",
		IconNameBrokenImage:       "broken_image",
		IconNameMailAttachment:    "attach_file",
		IconNameMailSend:          "send",
		IconNameMailReply:         "reply",
		IconNameMailReplyAll:      "reply_all",
		IconNameMailForward:       "mail_forward",
		IconNameMediaPhoto:        "photo",
		IconNameMediaVideo:        "video",
		IconNameMediaRecord:       "record",
		IconNameError:             "error",
		IconNameWarning:           "warning",
		IconNameHelp:              "help_outline",
		IconNameLogin:             "login",
		IconNameLogout:            "logout",
		IconNameCalendar:          "calendar",
		IconNameComputer:          "computer",
		IconNameDesktop:           "desktop",
		IconNameGrid:              "grid_view",
		IconNameViewFullScreen:    "fullscreen",
		IconNameViewRestore:       "fullscreen_exit",
		IconNameViewZoomIn:        "zoom_in",
		IconNameViewZoomOut:       "zoom_out",
		IconNameViewZoomFit:       "zoom_fit",
		IconNameColorPalette:      "palette",
		IconNameMoveUp:            "arrow_upward",
		IconNameMoveDown:          "arrow_downward",
		IconNameArrowDropDown:     "arrow_drop_down",
		IconNameArrowDropUp:       "arrow_drop_up",
		IconNameMenuExpand:        "expand_more",
	}
	snake := regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	seen := map[string]bool{}
	for name, w := range want {
		assert.Equal(t, w, string(name))
		assert.Regexp(t, snake, string(name))
		assert.False(t, seen[w], "%q names two icons", w)
		seen[w] = true
	}
	assert.Len(t, want, 107)
}

// A theme file written before #1307 overrides an icon by its old name,
// in JSON and in PXF; the new name wins when both are given; and "help"
// now overrides the question mark, not HelpIcon (help_outline).
func TestThemeFileOldIconNames(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	repository.Register("file", intRepo.NewFileRepository())
	const (
		cancel = "file://./testdata/cancel_Paths.svg"
		check  = "file://./testdata/check_GroupPaths.svg"
	)
	fromJSON, err := FromJSON(`{"Icons": {"mediaPlay": "` + cancel + `", "contentAdd": "` + cancel + `", "add": "` + check + `", "help": "` + cancel + `"}}`)
	require.NoError(t, err)
	fromPXF, err := FromPXF(`icons = { mediaPlay: "` + cancel + `"  contentAdd: "` + cancel + `"  add: "` + check + `"  help: "` + cancel + `" }`)
	require.NoError(t, err)
	for kind, th := range map[string]bellui.Theme{"json": fromJSON, "pxf": fromPXF} {
		assert.Equal(t, "cancel_Paths.svg", th.Icon(IconNameMediaPlay).Name(), "%s: old name mediaPlay", kind)
		assert.Equal(t, "check_GroupPaths.svg", th.Icon(IconNameContentAdd).Name(), "%s: new name add wins over contentAdd", kind)
		assert.Equal(t, "cancel_Paths.svg", th.Icon(IconNameQuestion).Name(), "%s: help is the question mark now", kind)
		assert.Equal(t, DefaultTheme().Icon(IconNameHelp).Name(), th.Icon(IconNameHelp).Name(), "%s: help no longer reaches HelpIcon", kind)
	}
}
