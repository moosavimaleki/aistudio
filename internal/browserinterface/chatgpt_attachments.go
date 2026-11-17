package browserinterface

import (
	"encoding/json"
	"fmt"

	"github.com/hamed/aistudio-api/internal/chatgptweb"
)

func chatAttachments(raw any) ([]chatgptweb.Attachment, error) {
	if raw == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var attachments []chatgptweb.Attachment
	if err := json.Unmarshal(encoded, &attachments); err != nil {
		return nil, err
	}
	if len(attachments) > 4 {
		return nil, fmt.Errorf("at most four reference images are supported")
	}
	for _, attachment := range attachments {
		if attachment.Name == "" || attachment.Data == "" || attachment.MIMEType == "" {
			return nil, fmt.Errorf("reference image data is incomplete")
		}
	}
	return attachments, nil
}
