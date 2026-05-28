package ews

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/johnkil/outlook-agent/internal/secret"
)

func BuildGetFolderRequest(config Config, password secret.Value, folderID string) (*http.Request, error) {
	endpoint, err := config.normalizedEndpointURL()
	if err != nil {
		return nil, err
	}
	body := getFolderEnvelope(folderID)
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	request.Header.Set("Accept", "text/xml")
	request.Header.Set("User-Agent", "outlook-agent")
	request.Header.Set("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/GetFolder")
	request.SetBasicAuth(config.Username, string(password))
	return request, nil
}

func BuildRawEWSRequest(config Config, password secret.Value, bodyXML string, soapAction string) (*http.Request, error) {
	endpoint, err := config.normalizedEndpointURL()
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(bodyXML))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	request.Header.Set("Accept", "text/xml")
	request.Header.Set("User-Agent", "outlook-agent")
	if strings.TrimSpace(soapAction) != "" {
		request.Header.Set("SOAPAction", strings.TrimSpace(soapAction))
	}
	request.SetBasicAuth(config.Username, string(password))
	return request, nil
}

func getFolderEnvelope(folderID string) string {
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	escapedFolderID := html.EscapeString(folderID)
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:GetFolder>
      <m:FolderShape>
        <t:BaseShape>Default</t:BaseShape>
      </m:FolderShape>
      <m:FolderIds>
        <t:DistinguishedFolderId Id="%s"/>
      </m:FolderIds>
    </m:GetFolder>
  </soap:Body>
</soap:Envelope>`, escapedFolderID)
}

type folderMetadata struct {
	DisplayName      string
	TotalCount       string
	ChildFolderCount string
	UnreadCount      string
	ResponseClass    string
	ResponseCode     string
	MessageText      string
}

func parseGetFolderResponse(reader io.Reader) (folderMetadata, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return folderMetadata{}, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var metadata folderMetadata
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return folderMetadata{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "GetFolderResponseMessage":
			for _, attr := range start.Attr {
				if attr.Name.Local == "ResponseClass" {
					metadata.ResponseClass = attr.Value
				}
			}
		case "ResponseCode":
			metadata.ResponseCode = readElementText(decoder, start)
		case "MessageText":
			metadata.MessageText = readElementText(decoder, start)
		case "DisplayName":
			metadata.DisplayName = readElementText(decoder, start)
		case "TotalCount":
			metadata.TotalCount = readElementText(decoder, start)
		case "ChildFolderCount":
			metadata.ChildFolderCount = readElementText(decoder, start)
		case "UnreadCount":
			metadata.UnreadCount = readElementText(decoder, start)
		}
	}
	if metadata.ResponseClass != "" && metadata.ResponseClass != "Success" {
		if metadata.MessageText != "" {
			return metadata, fmt.Errorf("ews GetFolder failed: %s", metadata.MessageText)
		}
		return metadata, fmt.Errorf("ews GetFolder failed: %s", metadata.ResponseCode)
	}
	if metadata.ResponseClass == "" && metadata.ResponseCode == "" && metadata.DisplayName == "" {
		return metadata, fmt.Errorf("missing GetFolder response")
	}
	return metadata, nil
}

func readElementText(decoder *xml.Decoder, start xml.StartElement) string {
	var value string
	if err := decoder.DecodeElement(&value, &start); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}
