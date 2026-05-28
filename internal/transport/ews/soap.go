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

func BuildFindItemRequest(config Config, password secret.Value, folderID string, maxItems int) (*http.Request, error) {
	endpoint, err := config.normalizedEndpointURL()
	if err != nil {
		return nil, err
	}
	body := findItemEnvelope(folderID, maxItems)
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	request.Header.Set("Accept", "text/xml")
	request.Header.Set("User-Agent", "outlook-agent")
	request.Header.Set("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/FindItem")
	request.SetBasicAuth(config.Username, string(password))
	return request, nil
}

func BuildGetItemRequest(config Config, password secret.Value, itemID string) (*http.Request, error) {
	endpoint, err := config.normalizedEndpointURL()
	if err != nil {
		return nil, err
	}
	body := getItemEnvelope(itemID)
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	request.Header.Set("Accept", "text/xml")
	request.Header.Set("User-Agent", "outlook-agent")
	request.Header.Set("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/GetItem")
	request.SetBasicAuth(config.Username, string(password))
	return request, nil
}

func BuildFindCalendarItemsRequest(config Config, password secret.Value, start string, end string, maxItems int) (*http.Request, error) {
	endpoint, err := config.normalizedEndpointURL()
	if err != nil {
		return nil, err
	}
	body := findCalendarItemsEnvelope(start, end, maxItems)
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	request.Header.Set("Accept", "text/xml")
	request.Header.Set("User-Agent", "outlook-agent")
	request.Header.Set("SOAPAction", "http://schemas.microsoft.com/exchange/services/2006/messages/FindItem")
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

func findItemEnvelope(folderID string, maxItems int) string {
	if strings.TrimSpace(folderID) == "" {
		folderID = "inbox"
	}
	if maxItems <= 0 {
		maxItems = 150
	}
	escapedFolderID := html.EscapeString(folderID)
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:FindItem Traversal="Shallow">
      <m:ItemShape>
        <t:BaseShape>IdOnly</t:BaseShape>
        <t:AdditionalProperties>
          <t:FieldURI FieldURI="item:Subject"/>
          <t:FieldURI FieldURI="message:From"/>
          <t:FieldURI FieldURI="item:DateTimeReceived"/>
          <t:FieldURI FieldURI="message:IsRead"/>
          <t:FieldURI FieldURI="item:HasAttachments"/>
        </t:AdditionalProperties>
      </m:ItemShape>
      <m:IndexedPageItemView MaxEntriesReturned="%d" Offset="0" BasePoint="Beginning"/>
      <m:ParentFolderIds>
        <t:DistinguishedFolderId Id="%s"/>
      </m:ParentFolderIds>
    </m:FindItem>
  </soap:Body>
</soap:Envelope>`, maxItems, escapedFolderID)
}

func getItemEnvelope(itemID string) string {
	escapedItemID := html.EscapeString(strings.TrimSpace(itemID))
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:GetItem>
      <m:ItemShape>
        <t:BaseShape>IdOnly</t:BaseShape>
        <t:AdditionalProperties>
          <t:FieldURI FieldURI="item:Subject"/>
          <t:FieldURI FieldURI="message:From"/>
          <t:FieldURI FieldURI="item:DateTimeReceived"/>
          <t:FieldURI FieldURI="message:IsRead"/>
          <t:FieldURI FieldURI="item:HasAttachments"/>
        </t:AdditionalProperties>
      </m:ItemShape>
      <m:ItemIds>
        <t:ItemId Id="%s"/>
      </m:ItemIds>
    </m:GetItem>
  </soap:Body>
</soap:Envelope>`, escapedItemID)
}

func findCalendarItemsEnvelope(start string, end string, maxItems int) string {
	if maxItems <= 0 {
		maxItems = 150
	}
	escapedStart := html.EscapeString(strings.TrimSpace(start))
	escapedEnd := html.EscapeString(strings.TrimSpace(end))
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
  xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
  <soap:Body>
    <m:FindItem Traversal="Shallow">
      <m:ItemShape>
        <t:BaseShape>IdOnly</t:BaseShape>
        <t:AdditionalProperties>
          <t:FieldURI FieldURI="item:Subject"/>
          <t:FieldURI FieldURI="calendar:Start"/>
          <t:FieldURI FieldURI="calendar:End"/>
          <t:FieldURI FieldURI="calendar:Location"/>
        </t:AdditionalProperties>
      </m:ItemShape>
      <m:CalendarView MaxEntriesReturned="%d" StartDate="%s" EndDate="%s"/>
      <m:ParentFolderIds>
        <t:DistinguishedFolderId Id="calendar"/>
      </m:ParentFolderIds>
    </m:FindItem>
  </soap:Body>
</soap:Envelope>`, maxItems, escapedStart, escapedEnd)
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

type findItemMessage struct {
	ID             string
	Subject        string
	FromName       string
	FromEmail      string
	ReceivedAt     string
	IsRead         bool
	HasAttachments bool
}

type calendarEvent struct {
	ID       string
	Subject  string
	Start    string
	End      string
	Location string
}

type findItemResponseEnvelope struct {
	Body struct {
		Response struct {
			ResponseMessages struct {
				Messages []findItemResponseMessage `xml:"FindItemResponseMessage"`
			} `xml:"ResponseMessages"`
		} `xml:"FindItemResponse"`
	} `xml:"Body"`
}

type findItemResponseMessage struct {
	ResponseClass string `xml:"ResponseClass,attr"`
	ResponseCode  string `xml:"ResponseCode"`
	MessageText   string `xml:"MessageText"`
	RootFolder    struct {
		Items struct {
			Messages []findItemMessageXML `xml:"Message"`
		} `xml:"Items"`
	} `xml:"RootFolder"`
}

type findItemMessageXML struct {
	ItemID struct {
		ID string `xml:"Id,attr"`
	} `xml:"ItemId"`
	Subject          string `xml:"Subject"`
	DateTimeReceived string `xml:"DateTimeReceived"`
	From             struct {
		Mailbox struct {
			Name         string `xml:"Name"`
			EmailAddress string `xml:"EmailAddress"`
		} `xml:"Mailbox"`
	} `xml:"From"`
	IsRead         string `xml:"IsRead"`
	HasAttachments string `xml:"HasAttachments"`
}

type getItemResponseEnvelope struct {
	Body struct {
		Response struct {
			ResponseMessages struct {
				Messages []getItemResponseMessage `xml:"GetItemResponseMessage"`
			} `xml:"ResponseMessages"`
		} `xml:"GetItemResponse"`
	} `xml:"Body"`
}

type getItemResponseMessage struct {
	ResponseClass string `xml:"ResponseClass,attr"`
	ResponseCode  string `xml:"ResponseCode"`
	MessageText   string `xml:"MessageText"`
	Items         struct {
		Messages []findItemMessageXML `xml:"Message"`
	} `xml:"Items"`
}

type findCalendarItemsResponseEnvelope struct {
	Body struct {
		Response struct {
			ResponseMessages struct {
				Messages []findCalendarItemsResponseMessage `xml:"FindItemResponseMessage"`
			} `xml:"ResponseMessages"`
		} `xml:"FindItemResponse"`
	} `xml:"Body"`
}

type findCalendarItemsResponseMessage struct {
	ResponseClass string `xml:"ResponseClass,attr"`
	ResponseCode  string `xml:"ResponseCode"`
	MessageText   string `xml:"MessageText"`
	RootFolder    struct {
		Items struct {
			CalendarItems []calendarItemXML `xml:"CalendarItem"`
		} `xml:"Items"`
	} `xml:"RootFolder"`
}

type calendarItemXML struct {
	ItemID struct {
		ID string `xml:"Id,attr"`
	} `xml:"ItemId"`
	Subject  string `xml:"Subject"`
	Start    string `xml:"Start"`
	End      string `xml:"End"`
	Location string `xml:"Location"`
}

func parseFindItemResponse(reader io.Reader) ([]findItemMessage, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var envelope findItemResponseEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Body.Response.ResponseMessages.Messages) == 0 {
		return nil, fmt.Errorf("missing FindItem response")
	}
	var messages []findItemMessage
	for _, response := range envelope.Body.Response.ResponseMessages.Messages {
		if response.ResponseClass != "" && response.ResponseClass != "Success" {
			if strings.TrimSpace(response.MessageText) != "" {
				return nil, fmt.Errorf("ews FindItem failed: %s", strings.TrimSpace(response.MessageText))
			}
			return nil, fmt.Errorf("ews FindItem failed: %s", strings.TrimSpace(response.ResponseCode))
		}
		for _, item := range response.RootFolder.Items.Messages {
			messages = append(messages, messageFromXML(item))
		}
	}
	if messages == nil {
		return []findItemMessage{}, nil
	}
	return messages, nil
}

func parseFindCalendarItemsResponse(reader io.Reader) ([]calendarEvent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var envelope findCalendarItemsResponseEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Body.Response.ResponseMessages.Messages) == 0 {
		return nil, fmt.Errorf("missing FindItem calendar response")
	}
	var events []calendarEvent
	for _, response := range envelope.Body.Response.ResponseMessages.Messages {
		if response.ResponseClass != "" && response.ResponseClass != "Success" {
			if strings.TrimSpace(response.MessageText) != "" {
				return nil, fmt.Errorf("ews FindItem calendar failed: %s", strings.TrimSpace(response.MessageText))
			}
			return nil, fmt.Errorf("ews FindItem calendar failed: %s", strings.TrimSpace(response.ResponseCode))
		}
		for _, item := range response.RootFolder.Items.CalendarItems {
			events = append(events, calendarEvent{
				ID:       strings.TrimSpace(item.ItemID.ID),
				Subject:  strings.TrimSpace(item.Subject),
				Start:    strings.TrimSpace(item.Start),
				End:      strings.TrimSpace(item.End),
				Location: strings.TrimSpace(item.Location),
			})
		}
	}
	if events == nil {
		return []calendarEvent{}, nil
	}
	return events, nil
}

func parseGetItemResponse(reader io.Reader) (findItemMessage, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return findItemMessage{}, err
	}
	var envelope getItemResponseEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return findItemMessage{}, err
	}
	if len(envelope.Body.Response.ResponseMessages.Messages) == 0 {
		return findItemMessage{}, fmt.Errorf("missing GetItem response")
	}
	for _, response := range envelope.Body.Response.ResponseMessages.Messages {
		if response.ResponseClass != "" && response.ResponseClass != "Success" {
			if strings.TrimSpace(response.MessageText) != "" {
				return findItemMessage{}, fmt.Errorf("ews GetItem failed: %s", strings.TrimSpace(response.MessageText))
			}
			return findItemMessage{}, fmt.Errorf("ews GetItem failed: %s", strings.TrimSpace(response.ResponseCode))
		}
		for _, item := range response.Items.Messages {
			return messageFromXML(item), nil
		}
	}
	return findItemMessage{}, fmt.Errorf("missing GetItem message")
}

func messageFromXML(item findItemMessageXML) findItemMessage {
	return findItemMessage{
		ID:             strings.TrimSpace(item.ItemID.ID),
		Subject:        strings.TrimSpace(item.Subject),
		FromName:       strings.TrimSpace(item.From.Mailbox.Name),
		FromEmail:      strings.TrimSpace(item.From.Mailbox.EmailAddress),
		ReceivedAt:     strings.TrimSpace(item.DateTimeReceived),
		IsRead:         strings.EqualFold(strings.TrimSpace(item.IsRead), "true"),
		HasAttachments: strings.EqualFold(strings.TrimSpace(item.HasAttachments), "true"),
	}
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
