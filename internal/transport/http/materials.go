// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/material"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// MaterialHandlers serves upload sessions and material versions (06 §4).
type MaterialHandlers struct {
	Materials *material.Service
}

func NewMaterialHandlers(svc *material.Service) *MaterialHandlers {
	return &MaterialHandlers{Materials: svc}
}

func (h *MaterialHandlers) Register(spec *SpecRouter) {
	spec.Register("listUploadSessions", withAuth(h.listSessions))
	spec.Register("createUploadSession", withAuth(h.createSession))
	spec.Register("uploadPart", withAuth(h.uploadPart))
	spec.Register("completeUpload", withAuth(h.complete))
	spec.Register("cancelUpload", withAuth(h.cancel))
	spec.Register("listMaterials", withAuth(h.listMaterials))
	spec.Register("listMaterialVersions", withAuth(h.listVersions))
	spec.Register("downloadMaterialContent", withAuth(h.download))
	spec.Register("createPreviewSession", withAuth(h.createPreview))
}

// PreviewOrigin configures where isolated previews live (empty = source
// download only, no interactive preview claims).
var PreviewOrigin string

func (h *MaterialHandlers) createPreview(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	materialID, err := uuid.Parse(c.Param("materialId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("materialId", "invalid"))
		return
	}
	versionID, err := uuid.Parse(c.Param("versionId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("versionId", "invalid"))
		return
	}
	session, err := h.Materials.CreatePreviewSession(c.Request.Context(), p.UserID, projectID, materialID, versionID, PreviewOrigin)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, session)
}

func (h *MaterialHandlers) listSessions(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	sessions, err := h.Materials.ListMySessions(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(sessions, nil))
}

func (h *MaterialHandlers) createSession(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Name       string `json:"name" binding:"required"`
		Size       int64  `json:"size" binding:"required"`
		SHA256     string `json:"sha256" binding:"required"`
		Mime       string `json:"mime"`
		Kind       string `json:"kind" binding:"required"`
		Entrypoint string `json:"entrypoint"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	session, err := h.Materials.CreateUpload(c.Request.Context(), p.UserID, projectID,
		req.Name, req.Kind, req.Mime, req.Size, req.SHA256, req.Entrypoint)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, session)
}

func (h *MaterialHandlers) uploadPart(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	uploadID, err := uuid.Parse(c.Param("uploadId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("uploadId", "invalid"))
		return
	}
	partNumber, err := strconv.Atoi(c.Param("partNumber"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("partNumber", "invalid"))
		return
	}
	declared := c.GetHeader("X-Judex-Part-SHA256")
	if declared == "" {
		// Fall back to computing the digest server-side when omitted; the
		// contract keeps the header required, but a missing one degrades to
		// server verification rather than rejection for web uploads.
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<30))
		if err != nil {
			respond{}.error(c, apierrors.New(apierrors.Validation, "body unreadable"))
			return
		}
		sum := sha256.Sum256(raw)
		declared = hex.EncodeToString(sum[:])
		c.Request.Body = io.NopCloser(newByteReader(raw))
	}
	written, err := h.Materials.UploadPart(c.Request.Context(), p.UserID, projectID, uploadID,
		partNumber, declared, c.Request.Body)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"partNumber": partNumber, "receivedBytes": written})
}

type byteReader struct {
	data []byte
	pos  int
}

func newByteReader(b []byte) *byteReader { return &byteReader{data: b} }

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (h *MaterialHandlers) complete(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	uploadID, err := uuid.Parse(c.Param("uploadId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("uploadId", "invalid"))
		return
	}
	var req struct {
		SourceVersion string `json:"sourceVersion"`
	}
	// Body is optional on complete.
	_ = c.ShouldBindJSON(&req)
	var sourceVersion *uuid.UUID
	if req.SourceVersion != "" {
		id, err := uuid.Parse(req.SourceVersion)
		if err != nil {
			respond{}.error(c, apierrors.Fields("sourceVersion", "invalid"))
			return
		}
		sourceVersion = &id
	}
	version, err := h.Materials.Complete(c.Request.Context(), p.UserID, projectID, uploadID, sourceVersion)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, version)
}

func (h *MaterialHandlers) cancel(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	uploadID, err := uuid.Parse(c.Param("uploadId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("uploadId", "invalid"))
		return
	}
	if err := h.Materials.Cancel(c.Request.Context(), p.UserID, projectID, uploadID); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"cancelled": true})
}

func (h *MaterialHandlers) listMaterials(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	materials, err := h.Materials.ListMaterials(c.Request.Context(), p.UserID, projectID, c.Query("kind"))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(materials, nil))
}

func (h *MaterialHandlers) listVersions(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	materialID, err := uuid.Parse(c.Param("materialId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("materialId", "invalid"))
		return
	}
	versions, err := h.Materials.ListVersions(c.Request.Context(), p.UserID, projectID, materialID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(versions, nil))
}

func (h *MaterialHandlers) download(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	materialID, err := uuid.Parse(c.Param("materialId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("materialId", "invalid"))
		return
	}
	versionID, err := uuid.Parse(c.Param("versionId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("versionId", "invalid"))
		return
	}
	entry := c.Query("entry")
	if entry == "" {
		entry = "part-000001"
	}
	body, _, err := h.Materials.VersionObject(c.Request.Context(), p.UserID, projectID, materialID, versionID, entry)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	defer body.Close()
	c.Header("Content-Disposition", "attachment")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(200)
	_, _ = io.Copy(c.Writer, body)
}
