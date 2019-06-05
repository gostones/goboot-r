package main

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"time"
)

// FileInfo provides blobstore file metadata
type FileInfo struct {
	Size     int64              `json:"size"`
	Type     string             `json:"type"`
	Symlink  string             `json:"symlink"`
	Version  string             `json:"version"`
	Modified time.Time          `json:"modified"`
	Metadata map[string]*string `json:"metadata"`
}

type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Info     *FileInfo   `json:"info,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

//types for the handlers
type DataAccess interface {
	Ls(path string) *FileNode
	Get(key string) (io.ReadCloser, error)
}

type fsDataAccess struct{}

func (da fsDataAccess) Ls(path string) *FileNode {
	return nil
}

func (da fsDataAccess) Get(key string) (io.ReadCloser, error) {
	return nil, nil
}

// Handlers
func HandleFsLsInternal() func(http.ResponseWriter, *http.Request) {
	return createHandleFsLs(fsDataAccess{})
}

func createHandleFsLs(dao DataAccess) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("### %v", r.URL)

		vars, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		val, pathExists := vars["path"]
		if !pathExists || val[0] == "" {
			http.Error(w, "URI not supported, ?path= missing ", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		root := dao.Ls(val[0])
		if err := json.NewEncoder(w).Encode(root); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func HandleFsGetInternal() func(http.ResponseWriter, *http.Request) {
	return createHandleFsGet(fsDataAccess{})
}

func createHandleFsGet(dao DataAccess) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pathname := r.FormValue("path")
		log.Debugf("The value of pathname is : %v", pathname)
		if len(pathname) == 0 {
			http.Error(w, "uri not supported, ?path= missing ", http.StatusBadRequest)
			return
		}
		var err error
		filterStr := r.FormValue("filter")
		log.Debugf("The value of fiterStr is : %v", filterStr)

		if filterStr != "" {
			filterStr, err = url.QueryUnescape(filterStr)
			log.Debugf("The value of inside fiterStr is : %v", filterStr)
			if err != nil {
				http.Error(w, "bad query "+filterStr, http.StatusBadRequest)
				return
			}
		}

		filename := path.Base(pathname)
		log.Debugf("The value of filename is : %v", filename)

		//
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", MimeTypeOfFile(filename))

		// get from blobstore
		resp, err := dao.Get(pathname)
		if err != nil {
			log.Debugf("@@@ get from blobstore: %v", err)

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Close()
		//
		_, err = io.Copy(w, resp)

		if err != nil {
			log.Debugf("@@@ get from blobstore decrypt: %v", err)

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Debugf("@@@ get from blobstore ok: %v", pathname)
	}
}

func MimeTypeOfFile(path string) string {
	return mime.TypeByExtension(filepath.Ext(path))
}
