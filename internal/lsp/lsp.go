// Package lsp is the language server. It receives the text of
// open documents from the editor, checks them with lint.CheckWithConfig and
// sends the findings back as diagnostics. The contract is in spec/README.md,
// "Language server contract".
package lsp
