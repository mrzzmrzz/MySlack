package main

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Quick Look shows files in the system's preview panel, as Messages shows
// a picture that is clicked: zoom, full screen, Open with Preview, share,
// and the arrows between the files. AppKit asks the responder chain who
// controls the panel; the app delegate answers, and hands it a data source
// of the files to show.

var (
	qlOnce    sync.Once
	qlOK      bool
	qlSource  objc.ID
	qlItems   []string // file paths
	qlCurrent int
)

func quickLookInit() {
	if _, err := purego.Dlopen("/System/Library/Frameworks/QuickLookUI.framework/QuickLookUI", purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
		log.Printf("quick look: %v", err)
		return
	}
	sel := objc.RegisterName
	nsURL := objc.ID(objc.GetClass("NSURL"))
	nsString := objc.ID(objc.GetClass("NSString"))

	// The data source: how many files, and each as a file URL.
	source, err := objc.RegisterClass("SMGPreviewSource", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{
			Cmd: sel("numberOfPreviewItemsInPreviewPanel:"),
			Fn: func(self objc.ID, _ objc.SEL, panel objc.ID) int {
				return len(qlItems)
			},
		},
		{
			Cmd: sel("previewPanel:previewItemAtIndex:"),
			Fn: func(self objc.ID, _ objc.SEL, panel objc.ID, i int) objc.ID {
				if i < 0 || i >= len(qlItems) {
					return 0
				}
				path := nsString.Send(sel("stringWithUTF8String:"), qlItems[i])
				return nsURL.Send(sel("fileURLWithPath:"), path)
			},
		},
	})
	if err != nil {
		log.Printf("quick look: %v", err)
		return
	}
	qlSource = objc.ID(source).Send(sel("new"))

	// The app delegate takes control of the panel when AppKit asks.
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	delegate := app.Send(sel("delegate")).Class()
	delegate.AddMethod(sel("acceptsPreviewPanelControl:"), objc.NewIMP(func(self objc.ID, _ objc.SEL, panel objc.ID) bool {
		return len(qlItems) > 0
	}), "B@:@")
	delegate.AddMethod(sel("beginPreviewPanelControl:"), objc.NewIMP(func(self objc.ID, _ objc.SEL, panel objc.ID) {
		panel.Send(sel("setDataSource:"), qlSource)
		panel.Send(sel("setCurrentPreviewItemIndex:"), qlCurrent)
	}), "v@:@")
	delegate.AddMethod(sel("endPreviewPanelControl:"), objc.NewIMP(func(self objc.ID, _ objc.SEL, panel objc.ID) {
		panel.Send(sel("setDataSource:"), objc.ID(0))
	}), "v@:@")
	qlOK = true
}

// quickLook shows the files at paths in the system's preview panel, the
// one at index first. It runs on the main thread, as AppKit needs.
func quickLook(paths []string, index int) bool {
	qlOnce.Do(quickLookInit)
	if !qlOK || len(paths) == 0 {
		return false
	}
	qlItems, qlCurrent = paths, index
	sel := objc.RegisterName
	panel := objc.ID(objc.GetClass("QLPreviewPanel")).Send(sel("sharedPreviewPanel"))
	if panel == 0 {
		return false
	}
	if panel.Send(sel("isVisible")) != 0 {
		panel.Send(sel("reloadData"))
		panel.Send(sel("setCurrentPreviewItemIndex:"), index)
		return true
	}
	panel.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	return true
}
