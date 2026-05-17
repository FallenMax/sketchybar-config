INSTALL_DIR = $(HOME)/.config/sketchybar

build:
	go build -o update_sketchybar -ldflags="-s -w" .

install: build
	mkdir -p $(INSTALL_DIR)
	rm -f $(INSTALL_DIR)/update_sketchybar
	cp update_sketchybar $(INSTALL_DIR)/
	cp sketchybarrc $(INSTALL_DIR)/
	chmod +x $(INSTALL_DIR)/sketchybarrc
	@if [ ! -f $(INSTALL_DIR)/config.json ]; then \
		cp config.default.json $(INSTALL_DIR)/config.json; \
		echo "created $(INSTALL_DIR)/config.json from defaults"; \
	else \
		echo "$(INSTALL_DIR)/config.json already exists, skipping"; \
	fi
	-$(INSTALL_DIR)/update_sketchybar teardown
	$(INSTALL_DIR)/update_sketchybar setup
	brew services restart sketchybar
	@sleep 2
	$(INSTALL_DIR)/update_sketchybar
	@echo ""
	@echo "✓ Installed to $(INSTALL_DIR)"
	@echo "  Edit $(INSTALL_DIR)/config.json to customize apps"

uninstall:
	$(INSTALL_DIR)/update_sketchybar teardown
	rm -f $(INSTALL_DIR)/update_sketchybar
	rm -f /tmp/sketchybar-update.lock
	brew services stop sketchybar
	@echo ""
	@echo "✓ Uninstalled. config.json and sketchybarrc left in place."
