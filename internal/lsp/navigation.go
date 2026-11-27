package lsp

import (
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// definition answers textDocument/definition: the locations where the name
// at the position is defined, or null.
func (server *server) definition(message incoming) error {
	var params textDocumentPositionParams
	if !server.decode(message, &params) {
		return server.connection.respondError(message.ID, codeInvalidParams, "invalid params")
	}
	open, found := server.pipelineDocument(params.TextDocument.URI)
	if !found {
		return server.connection.respond(message.ID, nil)
	}
	index := newLineIndex(open.text)
	pipeline, _ := lint.Parse([]byte(open.text))
	positions := navigate.Definition(pipeline, cursorOf(index, params.Position))
	return server.connection.respond(message.ID, toLocations(open.uri, positions, index))
}

// references answers textDocument/references: where the name at the
// position is used, with the definition if the client asks for it.
func (server *server) references(message incoming) error {
	var params referenceParams
	if !server.decode(message, &params) {
		return server.connection.respondError(message.ID, codeInvalidParams, "invalid params")
	}
	open, found := server.pipelineDocument(params.TextDocument.URI)
	if !found {
		return server.connection.respond(message.ID, nil)
	}
	index := newLineIndex(open.text)
	pipeline, _ := lint.Parse([]byte(open.text))
	positions := navigate.References(pipeline, cursorOf(index, params.Position), params.Context.IncludeDeclaration)
	return server.connection.respond(message.ID, toLocations(open.uri, positions, index))
}

// pipelineDocument returns a copy of an open document that is checked as a
// pipeline; the editor features answer only for those.
func (server *server) pipelineDocument(uri string) (document, bool) {
	server.mutex.Lock()
	open, isOpen := server.documents[uri]
	var snapshot document
	if isOpen {
		snapshot = *open
	}
	server.mutex.Unlock()
	if !isOpen || isConfigFile(snapshot.path) {
		return document{}, false
	}
	if snapshot.languageID != languageIDPipeline && !lint.LooksLikePipeline([]byte(snapshot.text)) {
		return document{}, false
	}
	return snapshot, true
}

func cursorOf(index lineIndex, position Position) model.Position {
	line, column := index.fromPosition(position)
	return model.Position{Line: line, Column: column}
}

// toLocations converts positions of the document; no positions is null,
// which editors show as "nothing found".
func toLocations(uri string, positions []model.Position, index lineIndex) []Location {
	if len(positions) == 0 {
		return nil
	}
	locations := make([]Location, 0, len(positions))
	for _, position := range positions {
		locations = append(locations, Location{URI: uri, Range: index.toRange(position)})
	}
	return locations
}
