// This file only keeps Go tooling (go ./..., gopls, golangci-lint) out of the
// frontend: some npm packages ship .go files inside node_modules.
module quorum/web

go 1.26
