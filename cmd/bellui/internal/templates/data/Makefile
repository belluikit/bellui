# If PREFIX isn't provided, we check for $(DESTDIR)/usr/local and use that if it exists.
# Otherwise we fall back to using /usr.

LOCAL != test -d $(DESTDIR)/usr/local && echo -n "/local" || echo -n ""
LOCAL ?= $(shell test -d $(DESTDIR)/usr/local && echo "/local" || echo "")
PREFIX ?= /usr$(LOCAL)

AppID := {{ .AppID | printf "%q" }}
Exec := {{ .Exec | printf "%q" }}
Icon := {{ .Icon | printf "%q" }}

default:
	# User install
	# Run "make user-install" to install in ~/.local/
	# Run "make user-uninstall" to uninstall from ~/.local/
	#
	# System install
	# Run "sudo make install" to install the application.
	# Run "sudo make uninstall" to uninstall the application.

install:
	install -Dm00644 usr/{{.Local}}share/applications/$(AppID).desktop $(DESTDIR)$(PREFIX)/share/applications/$(AppID).desktop
	install -Dm00755 usr/{{.Local}}bin/$(Exec) $(DESTDIR)$(PREFIX)/bin/$(Exec)
	install -Dm00644 usr/{{.Local}}share/pixmaps/$(Icon) $(DESTDIR)$(PREFIX)/share/pixmaps/$(Icon)
{{- if .Credits}}
	install -Dm00644 usr/{{.Local}}share/doc/$(AppID)/CREDITS $(DESTDIR)$(PREFIX)/share/doc/$(AppID)/CREDITS
{{- end}}
{{- if .BundleCEF}}
	# The CEF payload: the web engine this application bundles (bellui #107).
	mkdir -p $(DESTDIR)$(PREFIX)/bin/cef
	cp -r usr/{{.Local}}bin/cef/. $(DESTDIR)$(PREFIX)/bin/cef/
	# Chromium's sandbox needs its helper setuid root. Best effort: without
	# it the application still runs, with the sandbox disabled and a notice.
	-chown root:root $(DESTDIR)$(PREFIX)/bin/cef/chrome-sandbox && chmod 4755 $(DESTDIR)$(PREFIX)/bin/cef/chrome-sandbox
{{- end}}
uninstall:
	-rm $(DESTDIR)$(PREFIX)/share/applications/$(AppID).desktop
	-rm $(DESTDIR)$(PREFIX)/bin/$(Exec)
	-rm $(DESTDIR)$(PREFIX)/share/pixmaps/$(Icon)
{{- if .Credits}}
	-rm -r $(DESTDIR)$(PREFIX)/share/doc/$(AppID)
{{- end}}
{{- if .BundleCEF}}
	-rm -r $(DESTDIR)$(PREFIX)/bin/cef
{{- end}}

user-install:
	install -Dm00644 usr/{{.Local}}share/applications/$(AppID).desktop $(DESTDIR)$(HOME)/.local/share/applications/$(AppID).desktop
	install -Dm00755 usr/{{.Local}}bin/$(Exec) $(DESTDIR)$(HOME)/.local/bin/$(Exec)
	install -Dm00644 usr/{{.Local}}share/pixmaps/$(Icon) $(DESTDIR)$(HOME)/.local/share/icons/$(Icon)
	sed -i -e "s,Exec=$(Exec),Exec=$(DESTDIR)$(HOME)/.local/bin/$(Exec),g" $(DESTDIR)$(HOME)/.local/share/applications/$(AppID).desktop
{{- if .Credits}}
	install -Dm00644 usr/{{.Local}}share/doc/$(AppID)/CREDITS $(DESTDIR)$(HOME)/.local/share/doc/$(AppID)/CREDITS
{{- end}}
{{- if .BundleCEF}}
	# A user install cannot make the sandbox helper setuid root; the
	# application runs with the CEF sandbox disabled and says so.
	mkdir -p $(DESTDIR)$(HOME)/.local/bin/cef
	cp -r usr/{{.Local}}bin/cef/. $(DESTDIR)$(HOME)/.local/bin/cef/
{{- end}}

user-uninstall:
	-rm $(DESTDIR)$(HOME)/.local/share/applications/$(AppID).desktop
	-rm $(DESTDIR)$(HOME)/.local/bin/$(Exec)
	-rm $(DESTDIR)$(HOME)/.local/share/icons/$(Icon)
{{- if .Credits}}
	-rm -r $(DESTDIR)$(HOME)/.local/share/doc/$(AppID)
{{- end}}
{{- if .BundleCEF}}
	-rm -r $(DESTDIR)$(HOME)/.local/bin/cef
{{- end}}
