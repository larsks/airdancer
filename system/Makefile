sysconfdir=/etc
prefix=/usr/local
unitdir=$(sysconfdir)/systemd/system
cfgdir=$(sysconfdir)/airdancer
libexecdir=$(prefix)/libexec

UNITS = \
	airdancer-api.service \
	airdancer-caddy.service \
	airdancer-monitor.service \
	airdancer-ui.service \
	airdancer-wifi-fallback.service \
	airdancer.target

CFGFILES = \
	Caddyfile \
	airdancer-wifi-fallback.conf

SCRIPTS = \
	airdancer-wifi-fallback.sh

INSTALL = install
SYSTEMCTL = systemctl

all:

install:
	mkdir -p $(DESTDIR)$(cfgdir)
	for cfg in $(CFGFILES); do $(INSTALL) -m 644 $$cfg $(DESTDIR)$(cfgdir); done
	mkdir -p $(DESTDIR)$(unitdir)
	for unit in $(UNITS); do $(INSTALL) -m 644 $$unit $(DESTDIR)$(unitdir); done
	mkdir -p $(DESTDIR)$(libexecdir)
	for script in $(SCRIPTS); do $(INSTALL) -m 755 $$script $(DESTDIR)$(libexecdir); done

enable: install
	for unit in $(SERVICES); do $(SYSTEMCTL) enable $$unit; done

start: enable
	$(SYSTEMCTL) start airdancer.target
