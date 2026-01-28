package xrpc

import (
	"context"
	"io"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/lex/util"
)

// RepoUploadBlob calls the XRPC method "com.atproto.repo.uploadBlob".
func RepoUploadBlob(ctx context.Context, c util.LexClient, input io.Reader, contentType string) (*comatproto.RepoUploadBlob_Output, error) {
	var out comatproto.RepoUploadBlob_Output
	if err := c.LexDo(ctx, util.Procedure, contentType, "com.atproto.repo.uploadBlob", nil, input, &out); err != nil {
		return nil, err
	}

	return &out, nil
}
