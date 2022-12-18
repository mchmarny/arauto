package handler

import (
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

const (
	maxSeverityParamName = "severity"
	scanScopeParamName   = "scope"

	maxSeverityDefault = "critical"
	scanScopeDefault   = "squashed"
)

func (h *EventHandler) ScanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	log.Println("processing scan request...")

	if r.Method != http.MethodPost {
		writeError(w, errors.Errorf("method %s not supported, expected POST", r.Method))
		return
	}

	digest := r.URL.Query().Get(imageDigestQueryParamName)
	if digest == "" {
		writeError(w, errors.Errorf("verify %s parameter not set", imageDigestQueryParamName))
		return
	}

	maxSeverity := r.URL.Query().Get(maxSeverityParamName)
	if maxSeverity == "" {
		maxSeverity = maxSeverityDefault
	}

	scanScope := r.URL.Query().Get(scanScopeParamName)
	if scanScope == "" {
		scanScope = scanScopeDefault
	}

	sha, err := parseSHA(digest)
	if err != nil {
		writeError(w, errors.Wrap(err, "error parsing process event sha"))
		return
	}

	dir, err := makeFolder(sha)
	if err != nil {
		writeError(w, errors.Wrapf(err, "error creating context from sha: %s", sha))
		return
	}
	defer func() {
		if err = os.RemoveAll(dir); err != nil {
			log.Printf("error deleting context: %s\n", dir)
		}
	}()

	scanCmdArgs := append(h.scanCmdArgs, digest, maxSeverity, scanScope, dir)
	if err := runCommand(r.Context(), scanCmdArgs); err != nil {
		writeError(w, errors.Wrap(err, "error executing validation"))
		return
	}

	writeImageMessage(w, digest, "image scanned")
}
