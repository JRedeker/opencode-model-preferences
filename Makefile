.PHONY: build install clean

BINARY := omp
INSTALL_DIR := $(HOME)/.local/bin

build:
	go build -o $(BINARY) ./cmd/omp/

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

clean:
	rm -f $(BINARY)
