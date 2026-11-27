package lsp

import (
	"github.com/mfast99/concourse-lint/internal/complete"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/model"
)

// completion answers textDocument/completion: keys, names, keywords and
// artifacts at the position. The place comes from the current
// text, the names from the last version that could be read.
func (server *server) completion(message incoming) error {
	var params textDocumentPositionParams
	if !server.decode(message, &params) {
		return server.connection.respondError(message.ID, codeInvalidParams, "invalid params")
	}
	open, found := server.pipelineDocument(params.TextDocument.URI)
	if !found {
		return server.connection.respond(message.ID, nil)
	}
	index := newLineIndex(open.text)
	cursor := cursorOf(index, params.Position)
	result := complete.At([]byte(open.text), cursor, server.namesModel(open))
	start := index.toPosition(result.Start.Line, result.Start.Column)
	items := []completionItem{}
	for _, item := range result.Items {
		items = append(items, completionItem{
			Label:    item.Label,
			Kind:     completionKindOf(item.Kind),
			TextEdit: TextEdit{Range: Range{Start: start, End: params.Position}, NewText: item.Insert},
		})
	}
	return server.connection.respond(message.ID, items)
}

// namesModel returns the model of the document's text if it can be read,
// and remembers it; otherwise the last one that could be read.
func (server *server) namesModel(open document) *model.Pipeline {
	pipeline, readable := lint.Parse([]byte(open.text))
	server.mutex.Lock()
	defer server.mutex.Unlock()
	current, isOpen := server.documents[open.uri]
	if readable {
		if isOpen && current.text == open.text {
			current.lastModel = pipeline
		}
		return pipeline
	}
	if isOpen && current.lastModel != nil {
		return current.lastModel
	}
	return pipeline
}

func completionKindOf(kind complete.Kind) int {
	switch kind {
	case complete.KindField:
		return completionKindProperty
	case complete.KindKeyword:
		return completionKindKeyword
	}
	return completionKindReference
}
