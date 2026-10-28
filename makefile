INSTALL_DIR = $(HOME)/.config/sketchybar
CONFIG_FILE = $(INSTALL_DIR)/config.json
STAMP       = $(shell date +%Y%m%d-%H%M%S)

build:
	CGO_ENABLED=0 go build -o update_sketchybar -ldflags="-s -w" .

install: build
	@mkdir -p $(INSTALL_DIR)
	@if [ -f $(INSTALL_DIR)/sketchybarrc ] && ! grep -q "sketchybar-config" $(INSTALL_DIR)/sketchybarrc; then \
		cp $(INSTALL_DIR)/sketchybarrc $(INSTALL_DIR)/sketchybarrc.bak.$(STAMP); \
		echo "backed up your previous sketchybarrc to sketchybarrc.bak.$(STAMP)"; \
	fi
	install -m 755 update_sketchybar $(INSTALL_DIR)/update_sketchybar
	install -m 755 sketchybarrc $(INSTALL_DIR)/sketchybarrc
	install -m 755 yabai-hook.sh $(INSTALL_DIR)/yabai-hook.sh
	@if [ -f $(CONFIG_FILE) ]; then \
		echo "kept $(CONFIG_FILE)"; \
	else \
		cp config.default.json $(CONFIG_FILE); \
		echo "created $(CONFIG_FILE) from config.default.json"; \
	fi
	-$(INSTALL_DIR)/update_sketchybar setup
	@if brew services list 2>/dev/null | grep -q "^sketchybar *started"; then \
		brew services restart sketchybar; \
	else \
		brew services start sketchybar; \
	fi
	@sleep 2
	-$(INSTALL_DIR)/update_sketchybar
	@echo ""
	@$(INSTALL_DIR)/update_sketchybar summary
	@echo ""
	@echo "Verify the setup with: make doctor"

uninstall:
	-$(INSTALL_DIR)/update_sketchybar teardown
	-rm -f $(INSTALL_DIR)/update_sketchybar $(INSTALL_DIR)/yabai-hook.sh
	-brew services stop sketchybar
	@echo ""
	@echo "removed: $(INSTALL_DIR)/update_sketchybar, $(INSTALL_DIR)/yabai-hook.sh,"
	@echo "         the yabai signals, the two lines in $(HOME)/.yabairc,"
	@echo "         and $(HOME)/.cache/sketchybar-config"
	@echo "kept:    $(CONFIG_FILE), $(INSTALL_DIR)/sketchybarrc, and any *.bak.* backups"

doctor:
	@$(INSTALL_DIR)/update_sketchybar doctor

reload:
	sketchybar --reload

.PHONY: build install uninstall doctor reload
