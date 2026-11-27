package theme

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	col "github.com/belluikit/bellui/internal/color"
	"github.com/belluikit/bellui/internal/svg"
)

const (
	// IconNameCancel is the name of theme lookup for cancel icon.
	//
	// Since: 1.0
	IconNameCancel bellui.ThemeIconName = "cancel"

	// IconNameConfirm is the name of theme lookup for confirm icon.
	//
	// Since: 1.0
	IconNameConfirm bellui.ThemeIconName = "check"

	// IconNameDelete is the name of theme lookup for delete icon.
	//
	// Since: 1.0
	IconNameDelete bellui.ThemeIconName = "delete"

	// IconNameSearch is the name of theme lookup for search icon.
	//
	// Since: 1.0
	IconNameSearch bellui.ThemeIconName = "search"

	// IconNameSearchReplace is the name of theme lookup for search and replace icon.
	//
	// Since: 1.0
	IconNameSearchReplace bellui.ThemeIconName = "find_replace"

	// IconNameMenu is the name of theme lookup for menu icon.
	//
	// Since: 1.0
	IconNameMenu bellui.ThemeIconName = "menu"

	// IconNameMenuExpand is the name of theme lookup for menu expansion icon.
	//
	// Since: 1.0
	IconNameMenuExpand bellui.ThemeIconName = "expand_more"

	// IconNameCheckButton is the name of theme lookup for unchecked check button icon.
	//
	// Since: 1.0
	IconNameCheckButton bellui.ThemeIconName = "unchecked"

	// IconNameCheckButtonChecked is the name of theme lookup for checked check button icon.
	//
	// Since: 1.0
	IconNameCheckButtonChecked bellui.ThemeIconName = "checked"

	// IconNameCheckButtonFill is the name of theme lookup for filled check button icon.
	//
	// Since: 1.0
	IconNameCheckButtonFill bellui.ThemeIconName = "iconNameCheckButtonFill"

	// IconNameCheckButtonPartial is the name of theme lookup for "partially" checked check button icon.
	//
	// Since: 1.0
	IconNameCheckButtonPartial bellui.ThemeIconName = "partial"

	// IconNameRadioButton is the name of theme lookup for radio button unchecked icon.
	//
	// Since: 1.0
	IconNameRadioButton bellui.ThemeIconName = "radioButton"

	// IconNameRadioButtonChecked is the name of theme lookup for radio button checked icon.
	//
	// Since: 1.0
	IconNameRadioButtonChecked bellui.ThemeIconName = "radioButtonChecked"

	// IconNameRadioButtonFill is the name of theme lookup for filled radio button icon.
	//
	// Since: 1.0
	IconNameRadioButtonFill bellui.ThemeIconName = "iconNameRadioButtonFill"

	// IconNameColorAchromatic is the name of theme lookup for greyscale color icon.
	//
	// Since: 1.0
	IconNameColorAchromatic bellui.ThemeIconName = "colorAchromatic"

	// IconNameColorChromatic is the name of theme lookup for full color icon.
	//
	// Since: 1.0
	IconNameColorChromatic bellui.ThemeIconName = "colorChromatic"

	// IconNameColorPalette is the name of theme lookup for color palette icon.
	//
	// Since: 1.0
	IconNameColorPalette bellui.ThemeIconName = "palette"

	// IconNameContentAdd is the name of theme lookup for content add icon.
	//
	// Since: 1.0
	IconNameContentAdd bellui.ThemeIconName = "add"

	// IconNameContentRemove is the name of theme lookup for content remove icon.
	//
	// Since: 1.0
	IconNameContentRemove bellui.ThemeIconName = "remove"

	// IconNameContentCut is the name of theme lookup for content cut icon.
	//
	// Since: 1.0
	IconNameContentCut bellui.ThemeIconName = "cut"

	// IconNameContentCopy is the name of theme lookup for content copy icon.
	//
	// Since: 1.0
	IconNameContentCopy bellui.ThemeIconName = "content_copy"

	// IconNameContentPaste is the name of theme lookup for content paste icon.
	//
	// Since: 1.0
	IconNameContentPaste bellui.ThemeIconName = "paste"

	// IconNameContentClear is the name of theme lookup for content clear icon.
	//
	// Since: 1.0
	IconNameContentClear bellui.ThemeIconName = "clear"

	// IconNameContentRedo is the name of theme lookup for content redo icon.
	//
	// Since: 1.0
	IconNameContentRedo bellui.ThemeIconName = "redo"

	// IconNameContentUndo is the name of theme lookup for content undo icon.
	//
	// Since: 1.0
	IconNameContentUndo bellui.ThemeIconName = "undo"

	// IconNameInfo is the name of theme lookup for info icon.
	//
	// Since: 1.0
	IconNameInfo bellui.ThemeIconName = "info"

	// IconNameQuestion is the name of theme lookup for question icon.
	//
	// Since: 1.0
	IconNameQuestion bellui.ThemeIconName = "help"

	// IconNameWarning is the name of theme lookup for warning icon.
	//
	// Since: 1.0
	IconNameWarning bellui.ThemeIconName = "warning"

	// IconNameError is the name of theme lookup for error icon.
	//
	// Since: 1.0
	IconNameError bellui.ThemeIconName = "error"

	// IconNameBrokenImage is the name of the theme lookup for broken-image icon.
	//
	// Since: 1.0
	IconNameBrokenImage bellui.ThemeIconName = "broken_image"

	// IconNameDocument is the name of theme lookup for document icon.
	//
	// Since: 1.0
	IconNameDocument bellui.ThemeIconName = "document"

	// IconNameDocumentCreate is the name of theme lookup for document create icon.
	//
	// Since: 1.0
	IconNameDocumentCreate bellui.ThemeIconName = "edit"

	// IconNameDocumentPrint is the name of theme lookup for document print icon.
	//
	// Since: 1.0
	IconNameDocumentPrint bellui.ThemeIconName = "print"

	// IconNameDocumentSave is the name of theme lookup for document save icon.
	//
	// Since: 1.0
	IconNameDocumentSave bellui.ThemeIconName = "save"

	// IconNameDragCornerIndicator is the name of the icon used in inner windows to indicate a draggable corner.
	//
	// Since: 1.0
	IconNameDragCornerIndicator bellui.ThemeIconName = "dragCornerIndicator"

	// IconNameMoreHorizontal is the name of theme lookup for horizontal more.
	//
	// Since: 1.0
	IconNameMoreHorizontal bellui.ThemeIconName = "more_horiz"

	// IconNameMoreVertical is the name of theme lookup for vertical more.
	//
	// Since: 1.0
	IconNameMoreVertical bellui.ThemeIconName = "more"

	// IconNameMailAttachment is the name of theme lookup for mail attachment icon.
	//
	// Since: 1.0
	IconNameMailAttachment bellui.ThemeIconName = "attach_file"

	// IconNameMailCompose is the name of theme lookup for mail compose icon.
	//
	// Since: 1.0
	IconNameMailCompose bellui.ThemeIconName = "mail"

	// IconNameMailForward is the name of theme lookup for mail forward icon.
	//
	// Since: 1.0
	IconNameMailForward bellui.ThemeIconName = "mail_forward"

	// IconNameMailReply is the name of theme lookup for mail reply icon.
	//
	// Since: 1.0
	IconNameMailReply bellui.ThemeIconName = "reply"

	// IconNameMailReplyAll is the name of theme lookup for mail reply-all icon.
	//
	// Since: 1.0
	IconNameMailReplyAll bellui.ThemeIconName = "reply_all"

	// IconNameMailSend is the name of theme lookup for mail send icon.
	//
	// Since: 1.0
	IconNameMailSend bellui.ThemeIconName = "send"

	// IconNameMediaMusic is the name of theme lookup for media music icon.
	//
	// Since: 1.0
	IconNameMediaMusic bellui.ThemeIconName = "music"

	// IconNameMediaPhoto is the name of theme lookup for media photo icon.
	//
	// Since: 1.0
	IconNameMediaPhoto bellui.ThemeIconName = "photo"

	// IconNameMediaVideo is the name of theme lookup for media video icon.
	//
	// Since: 1.0
	IconNameMediaVideo bellui.ThemeIconName = "video"

	// IconNameMediaFastForward is the name of theme lookup for media fast-forward icon.
	//
	// Since: 1.0
	IconNameMediaFastForward bellui.ThemeIconName = "fast_forward"

	// IconNameMediaFastRewind is the name of theme lookup for media fast-rewind icon.
	//
	// Since: 1.0
	IconNameMediaFastRewind bellui.ThemeIconName = "fast_rewind"

	// IconNameMediaPause is the name of theme lookup for media pause icon.
	//
	// Since: 1.0
	IconNameMediaPause bellui.ThemeIconName = "pause"

	// IconNameMediaPlay is the name of theme lookup for media play icon.
	//
	// Since: 1.0
	IconNameMediaPlay bellui.ThemeIconName = "play"

	// IconNameMediaRecord is the name of theme lookup for media record icon.
	//
	// Since: 1.0
	IconNameMediaRecord bellui.ThemeIconName = "record"

	// IconNameMediaReplay is the name of theme lookup for media replay icon.
	//
	// Since: 1.0
	IconNameMediaReplay bellui.ThemeIconName = "replay"

	// IconNameMediaSkipNext is the name of theme lookup for media skip next icon.
	//
	// Since: 1.0
	IconNameMediaSkipNext bellui.ThemeIconName = "skip_next"

	// IconNameMediaSkipPrevious is the name of theme lookup for media skip previous icon.
	//
	// Since: 1.0
	IconNameMediaSkipPrevious bellui.ThemeIconName = "skip_previous"

	// IconNameMediaStop is the name of theme lookup for media stop icon.
	//
	// Since: 1.0
	IconNameMediaStop bellui.ThemeIconName = "stop"

	// IconNameMoveDown is the name of theme lookup for move down icon.
	//
	// Since: 1.0
	IconNameMoveDown bellui.ThemeIconName = "arrow_downward"

	// IconNameMoveUp is the name of theme lookup for move up icon.
	//
	// Since: 1.0
	IconNameMoveUp bellui.ThemeIconName = "arrow_upward"

	// IconNameNavigateBack is the name of theme lookup for navigate back icon.
	//
	// Since: 1.0
	IconNameNavigateBack bellui.ThemeIconName = "back"

	// IconNameNavigateNext is the name of theme lookup for navigate next icon.
	//
	// Since: 1.0
	IconNameNavigateNext bellui.ThemeIconName = "forward"

	// IconNameArrowDropDown is the name of theme lookup for drop-down arrow icon.
	//
	// Since: 1.0
	IconNameArrowDropDown bellui.ThemeIconName = "arrow_drop_down"

	// IconNameArrowDropUp is the name of theme lookup for drop-up arrow icon.
	//
	// Since: 1.0
	IconNameArrowDropUp bellui.ThemeIconName = "arrow_drop_up"

	// IconNameFile is the name of theme lookup for file icon.
	//
	// Since: 1.0
	IconNameFile bellui.ThemeIconName = "file"

	// IconNameFileApplication is the name of theme lookup for file application icon.
	//
	// Since: 1.0
	IconNameFileApplication bellui.ThemeIconName = "file_app"

	// IconNameFileAudio is the name of theme lookup for file audio icon.
	//
	// Since: 1.0
	IconNameFileAudio bellui.ThemeIconName = "file_audio"

	// IconNameFileImage is the name of theme lookup for file image icon.
	//
	// Since: 1.0
	IconNameFileImage bellui.ThemeIconName = "file_image"

	// IconNameFileText is the name of theme lookup for file text icon.
	//
	// Since: 1.0
	IconNameFileText bellui.ThemeIconName = "file_text"

	// IconNameFileVideo is the name of theme lookup for file video icon.
	//
	// Since: 1.0
	IconNameFileVideo bellui.ThemeIconName = "file_video"

	// IconNameFolder is the name of theme lookup for folder icon.
	//
	// Since: 1.0
	IconNameFolder bellui.ThemeIconName = "folder"

	// IconNameFolderNew is the name of theme lookup for folder new icon.
	//
	// Since: 1.0
	IconNameFolderNew bellui.ThemeIconName = "create_new_folder"

	// IconNameFolderOpen is the name of theme lookup for folder open icon.
	//
	// Since: 1.0
	IconNameFolderOpen bellui.ThemeIconName = "folder_open"

	// IconNameHelp is the name of theme lookup for help icon.
	//
	// Since: 1.0
	IconNameHelp bellui.ThemeIconName = "help_outline"

	// IconNameFavorite is the name of theme lookup for the filled favorite
	// (heart) icon, for a Like that is on.
	//
	// Since: 1.3
	IconNameFavorite bellui.ThemeIconName = "favorite"

	// IconNameFavoriteBorder is the name of theme lookup for the outlined
	// favorite (heart) icon, for a Like that is off.
	//
	// Since: 1.3
	IconNameFavoriteBorder bellui.ThemeIconName = "favorite_border"

	// IconNameBookmark is the name of theme lookup for the filled bookmark
	// icon, for a Save that is on.
	//
	// Since: 1.3
	IconNameBookmark bellui.ThemeIconName = "bookmark"

	// IconNameBookmarkBorder is the name of theme lookup for the outlined
	// bookmark icon, for a Save that is off.
	//
	// Since: 1.3
	IconNameBookmarkBorder bellui.ThemeIconName = "bookmark_border"

	// IconNameNotifications is the name of theme lookup for the outlined
	// notifications (bell) icon, for notices waiting to be read.
	//
	// Since: 1.3
	IconNameNotifications bellui.ThemeIconName = "notifications"

	// IconNameClosedCaption is the name of theme lookup for the filled
	// closed-captions icon, for captions that are on.
	//
	// Since: 1.3
	IconNameClosedCaption bellui.ThemeIconName = "closed_caption"

	// IconNameClosedCaptionOff is the name of theme lookup for the outlined
	// closed-captions icon, for captions that are off.
	//
	// Since: 1.3
	IconNameClosedCaptionOff bellui.ThemeIconName = "closed_caption_off"

	// IconNameShoppingCart is the name of theme lookup for Material's shopping_cart icon:
	// a shopping cart, for buying or a basket of items to buy (#1303).
	//
	// Since: 1.3
	IconNameShoppingCart bellui.ThemeIconName = "shopping_cart"

	// IconNameAddShoppingCart is the name of theme lookup for Material's add_shopping_cart icon:
	// a shopping cart with a plus, for adding an item to the cart (#1303).
	//
	// Since: 1.3
	IconNameAddShoppingCart bellui.ThemeIconName = "add_shopping_cart"

	// IconNameStorefront is the name of theme lookup for Material's storefront icon:
	// a shop front, for a store or the place to buy (#1303).
	//
	// Since: 1.3
	IconNameStorefront bellui.ThemeIconName = "storefront"

	// IconNameShoppingBag is the name of theme lookup for Material's shopping_bag icon:
	// a shopping bag, for purchases or a store (#1303).
	//
	// Since: 1.3
	IconNameShoppingBag bellui.ThemeIconName = "shopping_bag"

	// IconNameWorkspacePremium is the name of theme lookup for Material's workspace_premium icon:
	// a medal, for a premium or paid feature (#1303).
	//
	// Since: 1.3
	IconNameWorkspacePremium bellui.ThemeIconName = "workspace_premium"

	// IconNameDiamond is the name of theme lookup for Material's diamond icon:
	// a diamond, for a premium tier (#1303).
	//
	// Since: 1.3
	IconNameDiamond bellui.ThemeIconName = "diamond"

	// IconNameAutorenew is the name of theme lookup for Material's autorenew icon:
	// two arrows in a circle, for something that renews, such as a subscription (#1303).
	//
	// Since: 1.3
	IconNameAutorenew bellui.ThemeIconName = "autorenew"

	// IconNameCardMembership is the name of theme lookup for Material's card_membership icon:
	// a membership card, for a subscription or plan (#1303).
	//
	// Since: 1.3
	IconNameCardMembership bellui.ThemeIconName = "card_membership"

	// IconNameCreditCard is the name of theme lookup for Material's credit_card icon:
	// a credit card, for a payment method (#1303).
	//
	// Since: 1.3
	IconNameCreditCard bellui.ThemeIconName = "credit_card"

	// IconNameReceiptLong is the name of theme lookup for Material's receipt_long icon:
	// a long receipt, for a purchase history or an invoice (#1303).
	//
	// Since: 1.3
	IconNameReceiptLong bellui.ThemeIconName = "receipt_long"

	// IconNameLock is the name of theme lookup for Material's lock icon:
	// a closed padlock, for something locked, such as a paid feature (#1303).
	//
	// Since: 1.3
	IconNameLock bellui.ThemeIconName = "lock"

	// IconNameShare is the name of theme lookup for Material's share icon:
	// three connected dots, the share symbol most apps use (#1475).
	//
	// Since: 1.6
	IconNameShare bellui.ThemeIconName = "share"

	// IconNamePlace is the name of theme lookup for the icon of a place (map pin), for a destination about location.
	//
	// Since: 1.3
	IconNamePlace bellui.ThemeIconName = "place"

	// IconNamePeople is the name of theme lookup for the icon of people (a group), for a destination about friends or contacts.
	//
	// Since: 1.3
	IconNamePeople bellui.ThemeIconName = "people"

	// IconNameChild is the name of theme lookup for the icon of a child (a child's face), for a destination about children or kids.
	//
	// Since: 1.3
	IconNameChild bellui.ThemeIconName = "child"

	// IconNameHistory is the name of theme lookup for history icon.
	//
	// Since: 1.0
	IconNameHistory bellui.ThemeIconName = "history"

	// IconNameHome is the name of theme lookup for home icon.
	//
	// Since: 1.0
	IconNameHome bellui.ThemeIconName = "home"

	// IconNameSettings is the name of theme lookup for settings icon.
	//
	// Since: 1.0
	IconNameSettings bellui.ThemeIconName = "settings"

	// IconNameStorage is the name of theme lookup for storage icon.
	//
	// Since: 1.0
	IconNameStorage bellui.ThemeIconName = "storage"

	// IconNameUpload is the name of theme lookup for upload icon.
	//
	// Since: 1.0
	IconNameUpload bellui.ThemeIconName = "upload"

	// IconNameViewFullScreen is the name of theme lookup for view fullscreen icon.
	//
	// Since: 1.0
	IconNameViewFullScreen bellui.ThemeIconName = "fullscreen"

	// IconNameViewRefresh is the name of theme lookup for view refresh icon.
	//
	// Since: 1.0
	IconNameViewRefresh bellui.ThemeIconName = "refresh"

	// IconNameViewZoomFit is the name of theme lookup for view zoom fit icon.
	//
	// Since: 1.0
	IconNameViewZoomFit bellui.ThemeIconName = "zoom_fit"

	// IconNameViewZoomIn is the name of theme lookup for view zoom in icon.
	//
	// Since: 1.0
	IconNameViewZoomIn bellui.ThemeIconName = "zoom_in"

	// IconNameViewZoomOut is the name of theme lookup for view zoom out icon.
	//
	// Since: 1.0
	IconNameViewZoomOut bellui.ThemeIconName = "zoom_out"

	// IconNameViewRestore is the name of theme lookup for view restore icon.
	//
	// Since: 1.0
	IconNameViewRestore bellui.ThemeIconName = "fullscreen_exit"

	// IconNameVisibility is the name of theme lookup for visibility icon.
	//
	// Since: 1.0
	IconNameVisibility bellui.ThemeIconName = "visibility"

	// IconNameVisibilityOff is the name of theme lookup for invisibility icon.
	//
	// Since: 1.0
	IconNameVisibilityOff bellui.ThemeIconName = "visibility_off"

	// IconNameVolumeDown is the name of theme lookup for volume down icon.
	//
	// Since: 1.0
	IconNameVolumeDown bellui.ThemeIconName = "volume_down"

	// IconNameVolumeMute is the name of theme lookup for volume mute icon.
	//
	// Since: 1.0
	IconNameVolumeMute bellui.ThemeIconName = "volume_mute"

	// IconNameVolumeUp is the name of theme lookup for volume up icon.
	//
	// Since: 1.0
	IconNameVolumeUp bellui.ThemeIconName = "volume_up"

	// IconNameDownload is the name of theme lookup for download icon.
	//
	// Since: 1.0
	IconNameDownload bellui.ThemeIconName = "download"

	// IconNameComputer is the name of theme lookup for computer icon.
	//
	// Since: 1.0
	IconNameComputer bellui.ThemeIconName = "computer"

	// IconNameDesktop is the name of theme lookup for desktop icon.
	//
	// Since: 1.0
	IconNameDesktop bellui.ThemeIconName = "desktop"

	// IconNameAccount is the name of theme lookup for account icon.
	//
	// Since: 1.0
	IconNameAccount bellui.ThemeIconName = "account"

	// IconNameAccountCancel is the name of theme lookup for the account icon
	// with a block sign: a contact that is blocked, or will be (#1485).
	//
	// Since: 1.7
	IconNameAccountCancel bellui.ThemeIconName = "account_cancel"

	// IconNameCalendar is the name of theme lookup for calendar icon.
	//
	// Since: 1.0
	IconNameCalendar bellui.ThemeIconName = "calendar"

	// IconNameLogin is the name of theme lookup for login icon.
	//
	// Since: 1.0
	IconNameLogin bellui.ThemeIconName = "login"

	// IconNameLogout is the name of theme lookup for logout icon.
	//
	// Since: 1.0
	IconNameLogout bellui.ThemeIconName = "logout"

	// IconNameList is the name of theme lookup for list icon.
	//
	// Since: 1.0
	IconNameList bellui.ThemeIconName = "list"

	// IconNameGrid is the name of theme lookup for grid icon.
	//
	// Since: 1.0
	IconNameGrid bellui.ThemeIconName = "grid_view"

	// IconNameWindowClose is the name of theme lookup for window close icon.
	//
	// Since: 1.0
	IconNameWindowClose bellui.ThemeIconName = "windowClose"

	// IconNameWindowMaximize is the name of theme lookup for window maximize icon.
	//
	// Since: 1.0
	IconNameWindowMaximize bellui.ThemeIconName = "windowMaximize"

	// IconNameWindowMinimize is the name of theme lookup for window minimize icon.
	//
	// Since: 1.0
	IconNameWindowMinimize bellui.ThemeIconName = "windowMinimize"
)

var icons = map[bellui.ThemeIconName]bellui.Resource{
	IconNameCancel:        NewThemedResource(cancelIconRes),
	IconNameConfirm:       NewThemedResource(checkIconRes),
	IconNameDelete:        NewThemedResource(deleteIconRes),
	IconNameSearch:        NewThemedResource(searchIconRes),
	IconNameSearchReplace: NewThemedResource(searchreplaceIconRes),
	IconNameMenu:          NewThemedResource(menuIconRes),
	IconNameMenuExpand:    NewThemedResource(newDirectionalResource(menuexpandIconRes)),

	IconNameCheckButton:        NewThemedResource(checkboxIconRes),
	IconNameCheckButtonChecked: NewThemedResource(checkboxcheckedIconRes),
	IconNameCheckButtonFill:    NewThemedResource(checkboxfillIconRes),
	IconNameCheckButtonPartial: NewThemedResource(checkboxpartialIconRes),
	IconNameRadioButton:        NewThemedResource(radiobuttonIconRes),
	IconNameRadioButtonChecked: NewThemedResource(radiobuttoncheckedIconRes),
	IconNameRadioButtonFill:    NewThemedResource(radiobuttonfillIconRes),

	IconNameContentAdd:    NewThemedResource(contentaddIconRes),
	IconNameContentClear:  NewThemedResource(clearIconRes),
	IconNameContentRemove: NewThemedResource(contentremoveIconRes),
	IconNameContentCut:    NewThemedResource(contentcutIconRes),
	IconNameContentCopy:   NewThemedResource(contentcopyIconRes),
	IconNameContentPaste:  NewThemedResource(contentpasteIconRes),
	IconNameContentRedo:   NewThemedResource(contentredoIconRes),
	IconNameContentUndo:   NewThemedResource(contentundoIconRes),

	IconNameColorAchromatic: NewThemedResource(colorachromaticIconRes),
	IconNameColorChromatic:  NewThemedResource(colorchromaticIconRes),
	IconNameColorPalette:    NewThemedResource(colorpaletteIconRes),

	IconNameDocument:       NewThemedResource(documentIconRes),
	IconNameDocumentCreate: NewThemedResource(documentcreateIconRes),
	IconNameDocumentPrint:  NewThemedResource(documentprintIconRes),
	IconNameDocumentSave:   NewThemedResource(documentsaveIconRes),

	IconNameDragCornerIndicator: NewThemedResource(dragcornerindicatorIconRes),

	IconNameMoreHorizontal: NewThemedResource(morehorizontalIconRes),
	IconNameMoreVertical:   NewThemedResource(moreverticalIconRes),

	IconNameInfo:        NewThemedResource(infoIconRes),
	IconNameQuestion:    NewThemedResource(questionIconRes),
	IconNameWarning:     NewThemedResource(warningIconRes),
	IconNameError:       NewThemedResource(errorIconRes),
	IconNameBrokenImage: NewThemedResource(brokenimageIconRes),

	IconNameMailAttachment: NewThemedResource(mailattachmentIconRes),
	IconNameMailCompose:    NewThemedResource(mailcomposeIconRes),
	IconNameMailForward:    NewThemedResource(newDirectionalResource(mailforwardIconRes)),
	IconNameMailReply:      NewThemedResource(newDirectionalResource(mailreplyIconRes)),
	IconNameMailReplyAll:   NewThemedResource(newDirectionalResource(mailreplyallIconRes)),
	IconNameMailSend:       NewThemedResource(mailsendIconRes),

	IconNameMediaMusic:        NewThemedResource(mediamusicIconRes),
	IconNameMediaPhoto:        NewThemedResource(mediaphotoIconRes),
	IconNameMediaVideo:        NewThemedResource(mediavideoIconRes),
	IconNameMediaFastForward:  NewThemedResource(mediafastforwardIconRes),
	IconNameMediaFastRewind:   NewThemedResource(mediafastrewindIconRes),
	IconNameMediaPause:        NewThemedResource(mediapauseIconRes),
	IconNameMediaPlay:         NewThemedResource(mediaplayIconRes),
	IconNameMediaRecord:       NewThemedResource(mediarecordIconRes),
	IconNameMediaReplay:       NewThemedResource(mediareplayIconRes),
	IconNameMediaSkipNext:     NewThemedResource(mediaskipnextIconRes),
	IconNameMediaSkipPrevious: NewThemedResource(mediaskippreviousIconRes),
	IconNameMediaStop:         NewThemedResource(mediastopIconRes),

	IconNameNavigateBack:  NewThemedResource(newDirectionalResource(arrowbackIconRes)),
	IconNameMoveDown:      NewThemedResource(arrowdownIconRes),
	IconNameNavigateNext:  NewThemedResource(newDirectionalResource(arrowforwardIconRes)),
	IconNameMoveUp:        NewThemedResource(arrowupIconRes),
	IconNameArrowDropDown: NewThemedResource(arrowdropdownIconRes),
	IconNameArrowDropUp:   NewThemedResource(arrowdropupIconRes),

	IconNameFile:             NewThemedResource(fileIconRes),
	IconNameFileApplication:  NewThemedResource(fileapplicationIconRes),
	IconNameFileAudio:        NewThemedResource(fileaudioIconRes),
	IconNameFileImage:        NewThemedResource(fileimageIconRes),
	IconNameFileText:         NewThemedResource(filetextIconRes),
	IconNameFileVideo:        NewThemedResource(filevideoIconRes),
	IconNameFolder:           NewThemedResource(folderIconRes),
	IconNameFolderNew:        NewThemedResource(foldernewIconRes),
	IconNameFolderOpen:       NewThemedResource(folderopenIconRes),
	IconNameHelp:             NewThemedResource(helpIconRes),
	IconNameFavorite:         NewThemedResource(favoriteIconRes),
	IconNameFavoriteBorder:   NewThemedResource(favoriteborderIconRes),
	IconNameBookmark:         NewThemedResource(bookmarkIconRes),
	IconNameBookmarkBorder:   NewThemedResource(bookmarkborderIconRes),
	IconNameNotifications:    NewThemedResource(notificationsIconRes),
	IconNameClosedCaption:    NewThemedResource(closedcaptionIconRes),
	IconNameClosedCaptionOff: NewThemedResource(closedcaptionoffIconRes),
	IconNameShoppingCart:     NewThemedResource(shoppingcartIconRes),
	IconNameAddShoppingCart:  NewThemedResource(addshoppingcartIconRes),
	IconNameStorefront:       NewThemedResource(storefrontIconRes),
	IconNameShoppingBag:      NewThemedResource(shoppingbagIconRes),
	IconNameWorkspacePremium: NewThemedResource(workspacepremiumIconRes),
	IconNameDiamond:          NewThemedResource(diamondIconRes),
	IconNameAutorenew:        NewThemedResource(autorenewIconRes),
	IconNameCardMembership:   NewThemedResource(cardmembershipIconRes),
	IconNameCreditCard:       NewThemedResource(creditcardIconRes),
	IconNameReceiptLong:      NewThemedResource(receiptlongIconRes),
	IconNameLock:             NewThemedResource(lockIconRes),
	IconNameShare:            NewThemedResource(shareIconRes),
	IconNamePlace:            NewThemedResource(placeIconRes),
	IconNamePeople:           NewThemedResource(peopleIconRes),
	IconNameChild:            NewThemedResource(childIconRes),
	IconNameHistory:          NewThemedResource(historyIconRes),
	IconNameHome:             NewThemedResource(homeIconRes),
	IconNameSettings:         NewThemedResource(settingsIconRes),

	IconNameViewFullScreen: NewThemedResource(viewfullscreenIconRes),
	IconNameViewRefresh:    NewThemedResource(viewrefreshIconRes),
	IconNameViewRestore:    NewThemedResource(fullscreenexitIconRes),
	IconNameViewZoomFit:    NewThemedResource(viewzoomfitIconRes),
	IconNameViewZoomIn:     NewThemedResource(viewzoominIconRes),
	IconNameViewZoomOut:    NewThemedResource(viewzoomoutIconRes),

	IconNameVisibility:    NewThemedResource(visibilityIconRes),
	IconNameVisibilityOff: NewThemedResource(visibilityoffIconRes),

	IconNameVolumeDown: NewThemedResource(volumedownIconRes),
	IconNameVolumeMute: NewThemedResource(volumemuteIconRes),
	IconNameVolumeUp:   NewThemedResource(volumeupIconRes),

	IconNameDownload: NewThemedResource(downloadIconRes),
	IconNameComputer: NewThemedResource(computerIconRes),
	IconNameDesktop:  NewThemedResource(desktopIconRes),
	IconNameStorage:  NewThemedResource(storageIconRes),
	IconNameUpload:   NewThemedResource(uploadIconRes),

	IconNameAccount:       NewThemedResource(accountIconRes),
	IconNameAccountCancel: NewThemedResource(accountCancelIconRes),
	IconNameCalendar:      NewThemedResource(calendarIconRes),
	IconNameLogin:         NewThemedResource(loginIconRes),
	IconNameLogout:        NewThemedResource(logoutIconRes),

	IconNameList: NewThemedResource(listIconRes),
	IconNameGrid: NewThemedResource(gridIconRes),

	IconNameWindowClose:    NewThemedResource(cancelIconRes),
	IconNameWindowMaximize: NewThemedResource(maximizeIconRes),
	IconNameWindowMinimize: NewThemedResource(minimizeIconRes),
}

// Icon looks up the specified icon for current theme.
//
// Since: 1.0
func Icon(name bellui.ThemeIconName) bellui.Resource {
	return safeIconLookup(name)
}

// IconForWidget looks up the specified icon for requested widget using the current theme.
// If the widget theme has been overridden that theme will be used.
//
// Since: 1.0
func IconForWidget(name bellui.ThemeIconName, w core.Widget) bellui.Resource {
	return CurrentForWidget(w).Icon(name)
}

func (t *builtinTheme) Icon(n bellui.ThemeIconName) bellui.Resource {
	return icons[n]
}

var _ bellui.ThemedResource = (*ThemedResource)(nil)

// ThemedResource is a resource wrapper that will return a version of the resource with the main color changed
// for the currently selected theme.
type ThemedResource struct {
	source bellui.Resource

	// ColorName specifies which theme colour should be used to theme the resource
	//
	// Since: 1.0
	ColorName bellui.ThemeColorName
}

// NewColoredResource creates a resource that adapts to the current theme setting using
// the color named in the constructor.
//
// Since: 1.0
func NewColoredResource(src bellui.Resource, name bellui.ThemeColorName) *ThemedResource {
	return &ThemedResource{
		source:    src,
		ColorName: name,
	}
}

// NewColorizedResource creates a resource that draws an SVG icon in a fixed colour,
// whatever the theme, for an icon that must stand out against content of the app's own,
// such as artwork behind it. NewColoredResource is the theme-following counterpart.
// A themed source is tinted from its original SVG. A source that is not an SVG is
// returned unchanged.
func NewColorizedResource(src bellui.Resource, c color.Color) bellui.Resource {
	base := unwrapResource(src)
	if !svg.IsResourceSVG(base) {
		return src
	}
	r, g, b, a := col.ToNRGBA(c)
	return &colorizedResource{
		source:  base,
		name:    fmt.Sprintf("colorized_%02x%02x%02x%02x_%s", r, g, b, a, base.Name()),
		content: colorizeLogError(base.Content(), c),
	}
}

// colorizedResource is an SVG resource tinted once with a fixed colour.
type colorizedResource struct {
	source  bellui.Resource
	name    string
	content []byte
}

// Name returns the source's name with the colour, so each tint caches apart.
func (res *colorizedResource) Name() string {
	return res.name
}

// Content returns the tinted SVG.
func (res *colorizedResource) Content() []byte {
	return res.content
}

// NewSuccessThemedResource creates a resource that adapts to the current theme success color.
//
// Since: 1.0
func NewSuccessThemedResource(src bellui.Resource) *ThemedResource {
	return &ThemedResource{
		source:    src,
		ColorName: ColorNameSuccess,
	}
}

// NewThemedResource creates a resource that adapts to the current theme setting.
// By default, this will match the foreground color, but it can be changed using the `ColorName` field.
func NewThemedResource(src bellui.Resource) *ThemedResource {
	return &ThemedResource{
		source: src,
	}
}

// NewWarningThemedResource creates a resource that adapts to the current theme warning color.
//
// Since: 1.0
func NewWarningThemedResource(src bellui.Resource) *ThemedResource {
	return &ThemedResource{
		source:    src,
		ColorName: ColorNameWarning,
	}
}

// Name returns the underlying resource name (used for caching).
func (res *ThemedResource) Name() string {
	return string(res.ThemeColorName()) + "_" + unwrapResource(res.source).Name()
}

// ThemeColorName returns the bellui.ThemeColorName that is used as foreground color.
func (res *ThemedResource) ThemeColorName() bellui.ThemeColorName {
	if res.ColorName != "" {
		return res.ColorName
	}

	return ColorNameForeground
}

// Content returns the underlying content of the resource adapted to the current text color.
func (res *ThemedResource) Content() []byte {
	return colorizeLogError(unwrapResource(res.source).Content(), Color(res.ThemeColorName()))
}

// Error returns a different resource for indicating an error.
func (res *ThemedResource) Error() *ErrorThemedResource {
	return NewErrorThemedResource(res)
}

var _ bellui.ThemedResource = (*InvertedThemedResource)(nil)

// InvertedThemedResource is a resource wrapper that will return a version of the resource with the main color changed
// for use over highlighted elements.
type InvertedThemedResource struct {
	source bellui.Resource
}

// NewInvertedThemedResource creates a resource that adapts to the current theme for use over highlighted elements.
func NewInvertedThemedResource(orig bellui.Resource) *InvertedThemedResource {
	res := &InvertedThemedResource{source: orig}
	return res
}

// Name returns the underlying resource name (used for caching).
func (res *InvertedThemedResource) Name() string {
	return "inverted_" + unwrapResource(res.source).Name()
}

// Content returns the underlying content of the resource adapted to the current background color.
func (res *InvertedThemedResource) Content() []byte {
	clr := Color(ColorNameBackground)
	return colorizeLogError(unwrapResource(res.source).Content(), clr)
}

// ThemeColorName returns the bellui.ThemeColorName that is used as foreground color.
func (res *InvertedThemedResource) ThemeColorName() bellui.ThemeColorName {
	return ColorNameBackground
}

// Original returns the underlying resource that this inverted themed resource was adapted from
func (res *InvertedThemedResource) Original() bellui.Resource {
	return res.source
}

var _ bellui.ThemedResource = (*ErrorThemedResource)(nil)

// ErrorThemedResource is a resource wrapper that will return a version of the resource with the main color changed
// to indicate an error.
type ErrorThemedResource struct {
	source bellui.Resource
}

// NewErrorThemedResource creates a resource that adapts to the error color for the current theme.
func NewErrorThemedResource(orig bellui.Resource) *ErrorThemedResource {
	res := &ErrorThemedResource{source: orig}
	return res
}

// Name returns the underlying resource name (used for caching).
func (res *ErrorThemedResource) Name() string {
	return "error_" + unwrapResource(res.source).Name()
}

// Content returns the underlying content of the resource adapted to the current background color.
func (res *ErrorThemedResource) Content() []byte {
	return colorizeLogError(unwrapResource(res.source).Content(), Color(ColorNameError))
}

// Original returns the underlying resource that this error themed resource was adapted from
func (res *ErrorThemedResource) Original() bellui.Resource {
	return res.source
}

// ThemeColorName returns the bellui.ThemeColorName that is used as foreground color.
//
// Since: 1.0
func (res *ErrorThemedResource) ThemeColorName() bellui.ThemeColorName {
	return ColorNameError
}

var _ bellui.ThemedResource = (*PrimaryThemedResource)(nil)

// PrimaryThemedResource is a resource wrapper that will return a version of the resource with the main color changed
// to the theme primary color.
type PrimaryThemedResource struct {
	source bellui.Resource
}

// NewPrimaryThemedResource creates a resource that adapts to the primary color for the current theme.
func NewPrimaryThemedResource(orig bellui.Resource) *PrimaryThemedResource {
	return &PrimaryThemedResource{source: orig}
}

// Name returns the underlying resource name (used for caching).
func (res *PrimaryThemedResource) Name() string {
	return "primary_" + unwrapResource(res.source).Name()
}

// Content returns the underlying content of the resource adapted to the current background color.
func (res *PrimaryThemedResource) Content() []byte {
	return colorizeLogError(unwrapResource(res.source).Content(), Color(ColorNamePrimary))
}

// Original returns the underlying resource that this primary themed resource was adapted from
func (res *PrimaryThemedResource) Original() bellui.Resource {
	return res.source
}

// ThemeColorName returns the bellui.ThemeColorName that is used as foreground color.
//
// Since: 1.0
func (res *PrimaryThemedResource) ThemeColorName() bellui.ThemeColorName {
	return ColorNamePrimary
}

var _ bellui.ThemedResource = (*DisabledResource)(nil)

// DisabledResource is a resource wrapper that will return an appropriate resource colorized by
// the current theme's `DisabledColor` color.
type DisabledResource struct {
	source bellui.Resource
}

// Name returns the resource source name prefixed with `disabled_` (used for caching)
func (res *DisabledResource) Name() string {
	return "disabled_" + unwrapResource(res.source).Name()
}

// Content returns the disabled style content of the correct resource for the current theme
func (res *DisabledResource) Content() []byte {
	return colorizeLogError(unwrapResource(res.source).Content(), Color(ColorNameDisabled))
}

// ThemeColorName returns the bellui.ThemeColorName that is used as foreground color.
//
// Since: 1.0
func (res *DisabledResource) ThemeColorName() bellui.ThemeColorName {
	return ColorNameDisabled
}

// NewDisabledResource creates a resource that adapts to the current theme's DisabledColor setting.
func NewDisabledResource(res bellui.Resource) *DisabledResource {
	return &DisabledResource{
		source: res,
	}
}

// CancelIcon returns a resource containing the standard cancel icon for the current theme
func CancelIcon() bellui.Resource {
	return safeIconLookup(IconNameCancel)
}

// ConfirmIcon returns a resource containing the standard confirm icon for the current theme
func ConfirmIcon() bellui.Resource {
	return safeIconLookup(IconNameConfirm)
}

// DeleteIcon returns a resource containing the standard delete icon for the current theme
func DeleteIcon() bellui.Resource {
	return safeIconLookup(IconNameDelete)
}

// SearchIcon returns a resource containing the standard search icon for the current theme
func SearchIcon() bellui.Resource {
	return safeIconLookup(IconNameSearch)
}

// SearchReplaceIcon returns a resource containing the standard search and replace icon for the current theme
func SearchReplaceIcon() bellui.Resource {
	return safeIconLookup(IconNameSearchReplace)
}

// MenuIcon returns a resource containing the standard (mobile) menu icon for the current theme
func MenuIcon() bellui.Resource {
	return safeIconLookup(IconNameMenu)
}

// MenuExpandIcon returns a resource containing the standard (mobile) expand "submenu icon for the current theme
func MenuExpandIcon() bellui.Resource {
	return safeIconLookup(IconNameMenuExpand)
}

// CheckButtonIcon returns a resource containing the standard checkbox icon for the current theme
func CheckButtonIcon() bellui.Resource {
	return safeIconLookup(IconNameCheckButton)
}

// CheckButtonCheckedIcon returns a resource containing the standard checkbox checked icon for the current theme
func CheckButtonCheckedIcon() bellui.Resource {
	return safeIconLookup(IconNameCheckButtonChecked)
}

// CheckButtonFillIcon returns a resource containing the filled checkbox icon for the current theme.
//
// Since: 1.0
func CheckButtonFillIcon() bellui.Resource {
	return safeIconLookup(IconNameCheckButtonFill)
}

// RadioButtonIcon returns a resource containing the standard radio button icon for the current theme
func RadioButtonIcon() bellui.Resource {
	return safeIconLookup(IconNameRadioButton)
}

// RadioButtonCheckedIcon returns a resource containing the standard radio button checked icon for the current theme
func RadioButtonCheckedIcon() bellui.Resource {
	return safeIconLookup(IconNameRadioButtonChecked)
}

// RadioButtonFillIcon returns a resource containing the filled checkbox icon for the current theme.
//
// Since: 1.0
func RadioButtonFillIcon() bellui.Resource {
	return safeIconLookup(IconNameRadioButtonFill)
}

// ContentAddIcon returns a resource containing the standard content add icon for the current theme
func ContentAddIcon() bellui.Resource {
	return safeIconLookup(IconNameContentAdd)
}

// ContentRemoveIcon returns a resource containing the standard content remove icon for the current theme
func ContentRemoveIcon() bellui.Resource {
	return safeIconLookup(IconNameContentRemove)
}

// ContentClearIcon returns a resource containing the standard content clear icon for the current theme
func ContentClearIcon() bellui.Resource {
	return safeIconLookup(IconNameContentClear)
}

// ContentCutIcon returns a resource containing the standard content cut icon for the current theme
func ContentCutIcon() bellui.Resource {
	return safeIconLookup(IconNameContentCut)
}

// ContentCopyIcon returns a resource containing the standard content copy icon for the current theme
func ContentCopyIcon() bellui.Resource {
	return safeIconLookup(IconNameContentCopy)
}

// ContentPasteIcon returns a resource containing the standard content paste icon for the current theme
func ContentPasteIcon() bellui.Resource {
	return safeIconLookup(IconNameContentPaste)
}

// ContentRedoIcon returns a resource containing the standard content redo icon for the current theme
func ContentRedoIcon() bellui.Resource {
	return safeIconLookup(IconNameContentRedo)
}

// ContentUndoIcon returns a resource containing the standard content undo icon for the current theme
func ContentUndoIcon() bellui.Resource {
	return safeIconLookup(IconNameContentUndo)
}

// ColorAchromaticIcon returns a resource containing the standard achromatic color icon for the current theme
func ColorAchromaticIcon() bellui.Resource {
	return safeIconLookup(IconNameColorAchromatic)
}

// ColorChromaticIcon returns a resource containing the standard chromatic color icon for the current theme
func ColorChromaticIcon() bellui.Resource {
	return safeIconLookup(IconNameColorChromatic)
}

// ColorPaletteIcon returns a resource containing the standard color palette icon for the current theme
func ColorPaletteIcon() bellui.Resource {
	return safeIconLookup(IconNameColorPalette)
}

// DocumentIcon returns a resource containing the standard document icon for the current theme
func DocumentIcon() bellui.Resource {
	return safeIconLookup(IconNameDocument)
}

// DocumentCreateIcon returns a resource containing the standard document create icon for the current theme
func DocumentCreateIcon() bellui.Resource {
	return safeIconLookup(IconNameDocumentCreate)
}

// DocumentPrintIcon returns a resource containing the standard document print icon for the current theme
func DocumentPrintIcon() bellui.Resource {
	return safeIconLookup(IconNameDocumentPrint)
}

// DocumentSaveIcon returns a resource containing the standard document save icon for the current theme
func DocumentSaveIcon() bellui.Resource {
	return safeIconLookup(IconNameDocumentSave)
}

// MoreHorizontalIcon returns a resource containing the standard horizontal more icon for the current theme
func MoreHorizontalIcon() bellui.Resource {
	return Current().Icon(IconNameMoreHorizontal)
}

// MoreVerticalIcon returns a resource containing the standard vertical more icon for the current theme
func MoreVerticalIcon() bellui.Resource {
	return Current().Icon(IconNameMoreVertical)
}

// InfoIcon returns a resource containing the standard dialog info icon for the current theme
func InfoIcon() bellui.Resource {
	return safeIconLookup(IconNameInfo)
}

// QuestionIcon returns a resource containing the standard dialog question icon for the current theme
func QuestionIcon() bellui.Resource {
	return safeIconLookup(IconNameQuestion)
}

// WarningIcon returns a resource containing the standard dialog warning icon for the current theme
func WarningIcon() bellui.Resource {
	return safeIconLookup(IconNameWarning)
}

// ErrorIcon returns a resource containing the standard dialog error icon for the current theme
func ErrorIcon() bellui.Resource {
	return safeIconLookup(IconNameError)
}

// BrokenImageIcon returns a resource containing an icon to specify a broken or missing image
//
// Since: 1.0
func BrokenImageIcon() bellui.Resource {
	return safeIconLookup(IconNameBrokenImage)
}

// FileIcon returns a resource containing the appropriate file icon for the current theme
func FileIcon() bellui.Resource {
	return safeIconLookup(IconNameFile)
}

// FileApplicationIcon returns a resource containing the file icon representing application files for the current theme
func FileApplicationIcon() bellui.Resource {
	return safeIconLookup(IconNameFileApplication)
}

// FileAudioIcon returns a resource containing the file icon representing audio files for the current theme
func FileAudioIcon() bellui.Resource {
	return safeIconLookup(IconNameFileAudio)
}

// FileImageIcon returns a resource containing the file icon representing image files for the current theme
func FileImageIcon() bellui.Resource {
	return safeIconLookup(IconNameFileImage)
}

// FileTextIcon returns a resource containing the file icon representing text files for the current theme
func FileTextIcon() bellui.Resource {
	return safeIconLookup(IconNameFileText)
}

// FileVideoIcon returns a resource containing the file icon representing video files for the current theme
func FileVideoIcon() bellui.Resource {
	return safeIconLookup(IconNameFileVideo)
}

// FolderIcon returns a resource containing the standard folder icon for the current theme
func FolderIcon() bellui.Resource {
	return safeIconLookup(IconNameFolder)
}

// FolderNewIcon returns a resource containing the standard folder creation icon for the current theme
func FolderNewIcon() bellui.Resource {
	return safeIconLookup(IconNameFolderNew)
}

// FolderOpenIcon returns a resource containing the standard folder open icon for the current theme
func FolderOpenIcon() bellui.Resource {
	return safeIconLookup(IconNameFolderOpen)
}

// HelpIcon returns a resource containing the standard help icon for the current theme
func HelpIcon() bellui.Resource {
	return safeIconLookup(IconNameHelp)
}

// FavoriteIcon returns a resource containing the standard filled favorite
// (heart) icon for the current theme
//
// Since: 1.3
func FavoriteIcon() bellui.Resource {
	return safeIconLookup(IconNameFavorite)
}

// FavoriteBorderIcon returns a resource containing the standard outlined
// favorite (heart) icon for the current theme
//
// Since: 1.3
func FavoriteBorderIcon() bellui.Resource {
	return safeIconLookup(IconNameFavoriteBorder)
}

// BookmarkIcon returns a resource containing the standard filled bookmark
// icon for the current theme
//
// Since: 1.3
func BookmarkIcon() bellui.Resource {
	return safeIconLookup(IconNameBookmark)
}

// BookmarkBorderIcon returns a resource containing the standard outlined
// bookmark icon for the current theme
//
// Since: 1.3
func BookmarkBorderIcon() bellui.Resource {
	return safeIconLookup(IconNameBookmarkBorder)
}

// NotificationsIcon returns a resource containing the standard outlined
// notifications (bell) icon for the current theme
//
// Since: 1.3
func NotificationsIcon() bellui.Resource {
	return safeIconLookup(IconNameNotifications)
}

// ClosedCaptionIcon returns a resource containing the standard filled
// closed-captions icon for the current theme, for captions that are on
//
// Since: 1.3
func ClosedCaptionIcon() bellui.Resource {
	return safeIconLookup(IconNameClosedCaption)
}

// ClosedCaptionOffIcon returns a resource containing the standard outlined
// closed-captions icon for the current theme, for captions that are off
//
// Since: 1.3
func ClosedCaptionOffIcon() bellui.Resource {
	return safeIconLookup(IconNameClosedCaptionOff)
}

// ShoppingCartIcon returns a resource containing Material's shopping_cart icon for the
// current theme: a shopping cart, for buying or a basket of items to buy
//
// Since: 1.3
func ShoppingCartIcon() bellui.Resource {
	return safeIconLookup(IconNameShoppingCart)
}

// AddShoppingCartIcon returns a resource containing Material's add_shopping_cart icon for the
// current theme: a shopping cart with a plus, for adding an item to the cart
//
// Since: 1.3
func AddShoppingCartIcon() bellui.Resource {
	return safeIconLookup(IconNameAddShoppingCart)
}

// StorefrontIcon returns a resource containing Material's storefront icon for the
// current theme: a shop front, for a store or the place to buy
//
// Since: 1.3
func StorefrontIcon() bellui.Resource {
	return safeIconLookup(IconNameStorefront)
}

// ShoppingBagIcon returns a resource containing Material's shopping_bag icon for the
// current theme: a shopping bag, for purchases or a store
//
// Since: 1.3
func ShoppingBagIcon() bellui.Resource {
	return safeIconLookup(IconNameShoppingBag)
}

// WorkspacePremiumIcon returns a resource containing Material's workspace_premium icon for the
// current theme: a medal, for a premium or paid feature
//
// Since: 1.3
func WorkspacePremiumIcon() bellui.Resource {
	return safeIconLookup(IconNameWorkspacePremium)
}

// DiamondIcon returns a resource containing Material's diamond icon for the
// current theme: a diamond, for a premium tier
//
// Since: 1.3
func DiamondIcon() bellui.Resource {
	return safeIconLookup(IconNameDiamond)
}

// AutorenewIcon returns a resource containing Material's autorenew icon for the
// current theme: two arrows in a circle, for something that renews, such as a subscription
//
// Since: 1.3
func AutorenewIcon() bellui.Resource {
	return safeIconLookup(IconNameAutorenew)
}

// CardMembershipIcon returns a resource containing Material's card_membership icon for the
// current theme: a membership card, for a subscription or plan
//
// Since: 1.3
func CardMembershipIcon() bellui.Resource {
	return safeIconLookup(IconNameCardMembership)
}

// CreditCardIcon returns a resource containing Material's credit_card icon for the
// current theme: a credit card, for a payment method
//
// Since: 1.3
func CreditCardIcon() bellui.Resource {
	return safeIconLookup(IconNameCreditCard)
}

// ReceiptLongIcon returns a resource containing Material's receipt_long icon for the
// current theme: a long receipt, for a purchase history or an invoice
//
// Since: 1.3
func ReceiptLongIcon() bellui.Resource {
	return safeIconLookup(IconNameReceiptLong)
}

// LockIcon returns a resource containing Material's lock icon for the
// current theme: a closed padlock, for something locked, such as a paid feature
//
// Since: 1.3
func LockIcon() bellui.Resource {
	return safeIconLookup(IconNameLock)
}

// ShareIcon returns a resource containing Material's share icon for the
// current theme: three connected dots, the share symbol most apps use
//
// Since: 1.6
func ShareIcon() bellui.Resource {
	return safeIconLookup(IconNameShare)
}

// PlaceIcon returns a resource containing the standard icon of a place (map pin), for a destination about location, for the current theme
//
// Since: 1.3
func PlaceIcon() bellui.Resource {
	return safeIconLookup(IconNamePlace)
}

// PeopleIcon returns a resource containing the standard icon of people (a group), for a destination about friends or contacts, for the current theme
//
// Since: 1.3
func PeopleIcon() bellui.Resource {
	return safeIconLookup(IconNamePeople)
}

// ChildIcon returns a resource containing the standard icon of a child (a child's face), for a destination about children or kids, for the current theme
//
// Since: 1.3
func ChildIcon() bellui.Resource {
	return safeIconLookup(IconNameChild)
}

// HistoryIcon returns a resource containing the standard history icon for the current theme
func HistoryIcon() bellui.Resource {
	return safeIconLookup(IconNameHistory)
}

// HomeIcon returns a resource containing the standard home folder icon for the current theme
func HomeIcon() bellui.Resource {
	return safeIconLookup(IconNameHome)
}

// SettingsIcon returns a resource containing the standard settings icon for the current theme
func SettingsIcon() bellui.Resource {
	return safeIconLookup(IconNameSettings)
}

// MailAttachmentIcon returns a resource containing the standard mail attachment icon for the current theme
func MailAttachmentIcon() bellui.Resource {
	return safeIconLookup(IconNameMailAttachment)
}

// MailComposeIcon returns a resource containing the standard mail compose icon for the current theme
func MailComposeIcon() bellui.Resource {
	return safeIconLookup(IconNameMailCompose)
}

// MailForwardIcon returns a resource containing the standard mail forward icon for the current theme
func MailForwardIcon() bellui.Resource {
	return safeIconLookup(IconNameMailForward)
}

// MailReplyIcon returns a resource containing the standard mail reply icon for the current theme
func MailReplyIcon() bellui.Resource {
	return safeIconLookup(IconNameMailReply)
}

// MailReplyAllIcon returns a resource containing the standard mail reply all icon for the current theme
func MailReplyAllIcon() bellui.Resource {
	return safeIconLookup(IconNameMailReplyAll)
}

// MailSendIcon returns a resource containing the standard mail send icon for the current theme
func MailSendIcon() bellui.Resource {
	return safeIconLookup(IconNameMailSend)
}

// MediaMusicIcon returns a resource containing the standard media music icon for the current theme
//
// Since: 1.0
func MediaMusicIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaMusic)
}

// MediaPhotoIcon returns a resource containing the standard media photo icon for the current theme
//
// Since: 1.0
func MediaPhotoIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaPhoto)
}

// MediaVideoIcon returns a resource containing the standard media video icon for the current theme
//
// Since: 1.0
func MediaVideoIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaVideo)
}

// MediaFastForwardIcon returns a resource containing the standard media fast-forward icon for the current theme
func MediaFastForwardIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaFastForward)
}

// MediaFastRewindIcon returns a resource containing the standard media fast-rewind icon for the current theme
func MediaFastRewindIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaFastRewind)
}

// MediaPauseIcon returns a resource containing the standard media pause icon for the current theme
func MediaPauseIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaPause)
}

// MediaPlayIcon returns a resource containing the standard media play icon for the current theme
func MediaPlayIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaPlay)
}

// MediaRecordIcon returns a resource containing the standard media record icon for the current theme
func MediaRecordIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaRecord)
}

// MediaReplayIcon returns a resource containing the standard media replay icon for the current theme
func MediaReplayIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaReplay)
}

// MediaSkipNextIcon returns a resource containing the standard media skip next icon for the current theme
func MediaSkipNextIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaSkipNext)
}

// MediaSkipPreviousIcon returns a resource containing the standard media skip previous icon for the current theme
func MediaSkipPreviousIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaSkipPrevious)
}

// MediaStopIcon returns a resource containing the standard media stop icon for the current theme
func MediaStopIcon() bellui.Resource {
	return safeIconLookup(IconNameMediaStop)
}

// MoveDownIcon returns a resource containing the standard down arrow icon for the current theme
func MoveDownIcon() bellui.Resource {
	return safeIconLookup(IconNameMoveDown)
}

// MoveUpIcon returns a resource containing the standard up arrow icon for the current theme
func MoveUpIcon() bellui.Resource {
	return safeIconLookup(IconNameMoveUp)
}

// NavigateBackIcon returns a resource containing the standard backward navigation icon for the current theme
func NavigateBackIcon() bellui.Resource {
	return safeIconLookup(IconNameNavigateBack)
}

// NavigateNextIcon returns a resource containing the standard forward navigation icon for the current theme
func NavigateNextIcon() bellui.Resource {
	return safeIconLookup(IconNameNavigateNext)
}

// MenuDropDownIcon returns a resource containing the standard menu drop down icon for the current theme
func MenuDropDownIcon() bellui.Resource {
	return safeIconLookup(IconNameArrowDropDown)
}

// MenuDropUpIcon returns a resource containing the standard menu drop up icon for the current theme
func MenuDropUpIcon() bellui.Resource {
	return safeIconLookup(IconNameArrowDropUp)
}

// ViewFullScreenIcon returns a resource containing the standard fullscreen icon for the current theme
func ViewFullScreenIcon() bellui.Resource {
	return safeIconLookup(IconNameViewFullScreen)
}

// ViewRestoreIcon returns a resource containing the standard exit fullscreen icon for the current theme
func ViewRestoreIcon() bellui.Resource {
	return safeIconLookup(IconNameViewRestore)
}

// ViewRefreshIcon returns a resource containing the standard refresh icon for the current theme
func ViewRefreshIcon() bellui.Resource {
	return safeIconLookup(IconNameViewRefresh)
}

// ZoomFitIcon returns a resource containing the standard zoom fit icon for the current theme
func ZoomFitIcon() bellui.Resource {
	return safeIconLookup(IconNameViewZoomFit)
}

// ZoomInIcon returns a resource containing the standard zoom in icon for the current theme
func ZoomInIcon() bellui.Resource {
	return safeIconLookup(IconNameViewZoomIn)
}

// ZoomOutIcon returns a resource containing the standard zoom out icon for the current theme
func ZoomOutIcon() bellui.Resource {
	return safeIconLookup(IconNameViewZoomOut)
}

// VisibilityIcon returns a resource containing the standard visibility icon for the current theme
func VisibilityIcon() bellui.Resource {
	return safeIconLookup(IconNameVisibility)
}

// VisibilityOffIcon returns a resource containing the standard visibility off icon for the current theme
func VisibilityOffIcon() bellui.Resource {
	return safeIconLookup(IconNameVisibilityOff)
}

// VolumeDownIcon returns a resource containing the standard volume down icon for the current theme
func VolumeDownIcon() bellui.Resource {
	return safeIconLookup(IconNameVolumeDown)
}

// VolumeMuteIcon returns a resource containing the standard volume mute icon for the current theme
func VolumeMuteIcon() bellui.Resource {
	return safeIconLookup(IconNameVolumeMute)
}

// VolumeUpIcon returns a resource containing the standard volume up icon for the current theme
func VolumeUpIcon() bellui.Resource {
	return safeIconLookup(IconNameVolumeUp)
}

// ComputerIcon returns a resource containing the standard computer icon for the current theme
func ComputerIcon() bellui.Resource {
	return safeIconLookup(IconNameComputer)
}

// DesktopIcon returns a resource containing the standard desktop icon for the current theme
func DesktopIcon() bellui.Resource {
	return safeIconLookup(IconNameDesktop)
}

// DownloadIcon returns a resource containing the standard download icon for the current theme
func DownloadIcon() bellui.Resource {
	return safeIconLookup(IconNameDownload)
}

// StorageIcon returns a resource containing the standard storage icon for the current theme
func StorageIcon() bellui.Resource {
	return safeIconLookup(IconNameStorage)
}

// UploadIcon returns a resource containing the standard upload icon for the current theme
func UploadIcon() bellui.Resource {
	return safeIconLookup(IconNameUpload)
}

// AccountIcon returns a resource containing the standard account icon for the current theme
func AccountIcon() bellui.Resource {
	return safeIconLookup(IconNameAccount)
}

// AccountCancelIcon returns a resource containing the account icon with a
// block sign for the current theme: the person of AccountIcon with the
// "blocked" sign, for a contact that is blocked or will be (#1485).
//
// Since: 1.7
func AccountCancelIcon() bellui.Resource {
	return safeIconLookup(IconNameAccountCancel)
}

// CalendarIcon returns a resource containing the standard account icon for the current theme
//
// Since: 1.0
func CalendarIcon() bellui.Resource {
	return safeIconLookup(IconNameCalendar)
}

// LoginIcon returns a resource containing the standard login icon for the current theme
func LoginIcon() bellui.Resource {
	return safeIconLookup(IconNameLogin)
}

// LogoutIcon returns a resource containing the standard logout icon for the current theme
func LogoutIcon() bellui.Resource {
	return safeIconLookup(IconNameLogout)
}

// ListIcon returns a resource containing the standard list icon for the current theme
func ListIcon() bellui.Resource {
	return safeIconLookup(IconNameList)
}

// GridIcon returns a resource containing the standard grid icon for the current theme
func GridIcon() bellui.Resource {
	return safeIconLookup(IconNameGrid)
}

// WindowCloseIcon returns a resource containing the window close icon for the current theme
//
// Since: 1.0
func WindowCloseIcon() bellui.Resource {
	return safeIconLookup(IconNameWindowClose)
}

// WindowMaximizeIcon returns a resource containing the window maximize icon for the current theme
//
// Since: 1.0
func WindowMaximizeIcon() bellui.Resource {
	return safeIconLookup(IconNameWindowMaximize)
}

// WindowMinimizeIcon returns a resource containing the window minimize icon for the current theme
//
// Since: 1.0
func WindowMinimizeIcon() bellui.Resource {
	return safeIconLookup(IconNameWindowMinimize)
}

func safeIconLookup(n bellui.ThemeIconName) bellui.Resource {
	icon := Current().Icon(n)
	if icon == nil {
		bellui.LogError("Loaded theme returned nil icon", nil)
		return fallbackIcon
	}
	return icon
}

// recursively "unwraps" the source of the given resource to extract
// the "base" resource - to avoid recolorizing SVG multiple times
// if we for example have a ThemedResource wrapped in an ErrorThemedResource
func unwrapResource(res bellui.Resource) bellui.Resource {
	for {
		switch typedRes := res.(type) {
		case *DisabledResource:
			res = typedRes.source
		case *ErrorThemedResource:
			res = typedRes.source
		case *InvertedThemedResource:
			res = typedRes.source
		case *PrimaryThemedResource:
			res = typedRes.source
		case *ThemedResource:
			res = typedRes.source
		case *colorizedResource:
			res = typedRes.source
		default:
			return res
		}
	}
}

func colorizeLogError(src []byte, clr color.Color) []byte {
	content, err := svg.Colorize(src, clr)
	if err != nil {
		bellui.LogError("", err)
	}
	return content
}
