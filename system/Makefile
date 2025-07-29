sysconfdir=/etc
prefix=/usr/local
unitdir=$(sysconfdir)/systemd/system
cfgdir=$(sysconfdir)/airdancer
libexecdir=$(prefix)/libexec
nm_dispatch_dir=$(sysconfdir)/NetworkManager/dispatcher.d
udevdir=$(sysconfdir)/udev/rules.d

UNITS = \
	airdancer-api.service \
	airdancer-caddy.service \
	airdancer-monitor.service \
	airdancer-monitor@.service \
	airdancer-ui.service \
	airdancer-wifi-fallback.service \
	airdancer-buttons.service \
	airdancer-soundboard.service \
	airdancer-status.service \
	airdancer.target

CFGFILES = \
	Caddyfile \
	airdancer-wifi-fallback.conf \
	airdancer.env

UDEV_RULES = \
	wlan.rules

SCRIPTS = \
	airdancer-wifi-fallback.sh

NM_DISPATCH_SCRIPTS = \
	update-noip-ddns.sh

INSTALL = install
SYSTEMCTL = systemctl

all:

install: install-config install-units install-scripts install-nm-dispatch-scripts install-udev-rules

install-config:
	mkdir -p $(DESTDIR)$(cfgdir)
	for cfg in $(CFGFILES); do $(INSTALL) -m 644 $$cfg $(DESTDIR)$(cfgdir); done

install-units:
	mkdir -p $(DESTDIR)$(unitdir)
	for unit in $(UNITS); do $(INSTALL) -m 644 $$unit $(DESTDIR)$(unitdir); done

install-scripts:
	mkdir -p $(DESTDIR)$(libexecdir)
	for script in $(SCRIPTS); do $(INSTALL) -m 755 $$script $(DESTDIR)$(libexecdir); done

install-nm-dispatch-scripts:
	mkdir -p $(DESTDIR)$(nm_dispatch_dir)
	for script in $(NM_DISPATCH_SCRIPTS); do $(INSTALL) -m 755 $$script $(DESTDIR)$(nm_dispatch_dir); done

install-udev-rules:
	mkdir -p $(DESTDIR)$(udevdir)
	for rules in $(UDEV_RULES); do $(INSTALL) -m 644 $$rules $(DESTDIR)$(udevdir); done
