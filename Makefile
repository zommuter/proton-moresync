SYSTEMD_USER := $(HOME)/.config/systemd/user
CONFIG_DIR   := $(HOME)/.config/proton-moresync
ENV_FILE     := $(CONFIG_DIR)/env

.PHONY: build install enable disable uninstall

build:
	go build -o backup ./cmd/backup

# Create the per-user env file (if absent), copy units, reload, enable timer.
install: $(ENV_FILE)
	cp systemd/proton-backup.service $(SYSTEMD_USER)/proton-backup.service
	cp systemd/proton-backup.timer   $(SYSTEMD_USER)/proton-backup.timer
	systemctl --user daemon-reload
	@echo "Run 'make enable' to start the daily timer."

# Prompt for values only when the env file does not yet exist.
$(ENV_FILE):
	@mkdir -p $(CONFIG_DIR)
	@printf 'PROTON_USER='; read u; \
	 printf 'PROTON_BACKUP_DIR='; read d; \
	 printf 'PROTON_USER=%s\nPROTON_BACKUP_DIR=%s\n' "$$u" "$$d" > $(ENV_FILE)
	@echo "Wrote $(ENV_FILE)"

enable:
	systemctl --user enable --now proton-backup.timer

disable:
	systemctl --user disable --now proton-backup.timer

uninstall: disable
	rm -f $(SYSTEMD_USER)/proton-backup.service $(SYSTEMD_USER)/proton-backup.timer
	systemctl --user daemon-reload
