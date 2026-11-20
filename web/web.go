// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html log.html panels.html paneleditor.html panel.html panels-logo.svg *.js base.css desktop.css mobile.css history.css exportimport.css log.css panels.css panelmon.css gridpanel.css gridpanelphone.css gridpopup.css lock.css touch.css setup.css approvalcards.css palette.css vendor
var FS embed.FS
