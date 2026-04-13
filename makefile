INSTALL_DIR = $(HOME)/.config/sketchybar

build:
	go build -o update_sketchybar -ldflags="-s -w" .

install: build
	mkdir -p $(INSTALL_DIR)
	cp update_sketchybar $(INSTALL_DIR)/
	cp sketchybarrc $(INSTALL_DIR)/
	chmod +x $(INSTALL_DIR)/sketchybarrc
	$(INSTALL_DIR)/update_sketchybar setup
	brew services restart sketchybar
	@sleep 2
	$(INSTALL_DIR)/update_sketchybar
	@echo ""
	@echo "✓ Installed to $(INSTALL_DIR)"

uninstall:
	$(INSTALL_DIR)/update_sketchybar teardown
	rm -f $(INSTALL_DIR)/update_sketchybar
	rm -f $(INSTALL_DIR)/update_sketchybar.lock
	brew services stop sketchybar
	@echo ""
	@echo "✓ Uninstalled. sketchybarrc left in place — remove manually if desired."
