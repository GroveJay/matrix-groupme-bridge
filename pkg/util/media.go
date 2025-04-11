package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var UserAgent = "groupme/0.1.0 libgroupme/" + "0" + " go/" + strings.TrimPrefix(runtime.Version(), "go")

var mediaHTTPClient = http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ForceAttemptHTTP2:     true,
	},
	Timeout: 120 * time.Second,
}

var ErrTooLargeFile = bridgev2.WrapErrorInStatus(errors.New("too large file")).
	WithErrorAsMessage().WithSendNotice(true).WithErrorReason(event.MessageStatusUnsupported)
var ErrURLNotFound = errors.New("url not found")

func addDownloadHeaders(hdr http.Header, mime string) {
	hdr.Set("Accept", "*/*")
	switch strings.Split(mime, "/")[0] {
	case "image":
		hdr.Set("Accept", "image/avif,image/webp,*/*")
		hdr.Set("Sec-Fetch-Dest", "image")
	case "video":
		hdr.Set("Sec-Fetch-Dest", "video")
	case "audio":
		hdr.Set("Sec-Fetch-Dest", "audio")
	default:
		hdr.Set("Sec-Fetch-Dest", "empty")
	}
	hdr.Set("Sec-Fetch-Mode", "no-cors")
	hdr.Set("Sec-Fetch-Site", "cross-site")
	// Setting a referer seems to disable redirects for some reason
	//hdr.Set("Referer", MediaReferer)
	hdr.Set("User-Agent", UserAgent)
	//hdr.Set("sec-ch-ua", messagix.SecCHUserAgent)
	//hdr.Set("sec-ch-ua-platform", messagix.SecCHPlatform)
}

func DownloadMedia(ctx context.Context, mime, url string, maxSize int64, byteRange string, switchToChunked bool) (int64, io.ReadCloser, error) {
	zerolog.Ctx(ctx).Trace().Str("url", url).Msg("Downloading media")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	addDownloadHeaders(req.Header, mime)
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}

	resp, err := mediaHTTPClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to send request: %w", err)
	} else if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		_ = resp.Body.Close()
		/*if resp.StatusCode == 302 && switchToChunked {
			loc, _ := resp.Location()
			if loc != nil && loc.Hostname() == "video.xx.fbcdn.net" {
				return downloadChunkedVideo(ctx, mime, loc.String(), maxSize)
			}
		}*/
		return 0, nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	} else if resp.ContentLength > maxSize {
		_ = resp.Body.Close()
		return resp.ContentLength, nil, fmt.Errorf("%w (%.2f MiB)", ErrTooLargeFile, float64(resp.ContentLength)/1024/1024)
	}
	zerolog.Ctx(ctx).Debug().Int64("content_length", resp.ContentLength).Msg("Got media response")
	return resp.ContentLength, resp.Body, nil
}

// DownloadImage helper function to download image from groupme;
// append .large/.preview/.avatar to get various sizes
func DownloadImage(URL string) (bytes *[]byte, mime string, err error) {
	response, err := http.Get(URL)
	if err != nil {
		return nil, "", errors.New("Failed to download avatar: " + err.Error())
	}
	defer response.Body.Close()

	image, err := io.ReadAll(response.Body)
	bytes = &image
	if err != nil {
		return nil, "", errors.New("Failed to read downloaded image:" + err.Error())
	}

	mime = response.Header.Get("Content-Type")
	if len(mime) == 0 {
		mime = http.DetectContentType(image)
	}
	return
}

func ConvertAttachment(ctx context.Context, attachment *groupmeclient.Attachment, intent bridgev2.MatrixAPI, roomId id.RoomID) (MessageEventContent *event.MessageEventContent, err error) {
	// TODO: Other attachment types: mentions, location, emoji(?), video?
	switch attachment.Type {
	case groupmeclient.Image:
		imgData, mime, err := DownloadImage(attachment.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to load media info: %w", err)
		}
		var width, height int
		if strings.HasPrefix(mime, "image/") {
			cfg, _, _ := image.DecodeConfig(bytes.NewReader(*imgData))
			width, height = cfg.Width, cfg.Height
		}
		fileName := GetGroupmeFilename(attachment.URL)
		url, file, err := intent.UploadMedia(ctx, roomId, *imgData, fileName, mime)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", bridgev2.ErrMediaReuploadFailed, err)
		}
		return &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    fileName,
			URL:     url,
			Info: &event.FileInfo{
				MimeType: mime,
				Size:     len(*imgData),
				Height:   height,
				Width:    width,
			},
			File: file,
		}, nil
	default:
		return nil, fmt.Errorf("unable to handle groupme attachment type %s", attachment.Type)
	}
	/*
	   case "video":

	   	vidContents, mime := groupmeext.DownloadVideo(attachment.VideoPreviewURL, attachment.URL, source.Token)
	   	if mime == "" {
	   		mime = mimetype.Detect(vidContents).String()
	   	}

	   	data, uploadMimeType, file := portal.encryptFile(vidContents, mime)
	   	uploaded, err := intent.UploadBytes(data, uploadMimeType)
	   	if err != nil {
	   		if errors.Is(err, mautrix.MTooLarge) {
	   			err = errors.New("homeserver rejected too large file")
	   		} else if httpErr := err.(mautrix.HTTPError); httpErr.IsStatus(413) {
	   			err = errors.New("proxy rejected too large file")
	   		} else {
	   			err = fmt.Errorf("failed to upload media: %w", err)
	   		}
	   		return nil, true, err
	   	}

	   	text := strings.Split(attachment.URL, "/")
	   	content := &event.MessageEventContent{
	   		Body: text[len(text)-1],
	   		File: file,
	   		Info: &event.FileInfo{
	   			Size:     len(data),
	   			MimeType: mime,
	   			//Width:    width,
	   			//Height:   height,
	   			//Duration: int(msg.length),
	   		},
	   	}
	   	if content.File != nil {
	   		content.File.URL = uploaded.ContentURI.CUString()
	   	} else {
	   		content.URL = uploaded.ContentURI.CUString()
	   	}
	   	content.MsgType = event.MsgVideo

	   	message.Text = strings.Replace(message.Text, attachment.URL, "", 1)
	   	return content, true, nil

	   case "file":

	   	fileData, fname, fmime := groupmeext.DownloadFile(portal.Key.GMID, attachment.FileID, source.Token)
	   	if fmime == "" {
	   		fmime = mimetype.Detect(fileData).String()
	   	}
	   	data, uploadMimeType, file := portal.encryptFile(fileData, fmime)

	   	uploaded, err := intent.UploadBytes(data, uploadMimeType)
	   	if err != nil {
	   		if errors.Is(err, mautrix.MTooLarge) {
	   			err = errors.New("homeserver rejected too large file")
	   		} else if httpErr := err.(mautrix.HTTPError); httpErr.IsStatus(413) {
	   			err = errors.New("proxy rejected too large file")
	   		} else {
	   			err = fmt.Errorf("failed to upload media: %w", err)
	   		}
	   		return nil, true, err
	   	}

	   	content := &event.MessageEventContent{
	   		Body: fname,
	   		File: file,
	   		Info: &event.FileInfo{
	   			Size:     len(data),
	   			MimeType: fmime,
	   			//Width:    width,
	   			//Height:   height,
	   			//Duration: int(msg.length),
	   		},
	   	}
	   	if content.File != nil {
	   		content.File.URL = uploaded.ContentURI.CUString()
	   	} else {
	   		content.URL = uploaded.ContentURI.CUString()
	   	}
	   	//TODO thumbnail since groupme supports it anyway
	   	if strings.HasPrefix(fmime, "image") {
	   		content.MsgType = event.MsgImage
	   	} else if strings.HasPrefix(fmime, "video") {
	   		content.MsgType = event.MsgVideo
	   	} else {
	   		content.MsgType = event.MsgFile
	   	}

	   	return content, false, nil

	   case "location":

	   	name := attachment.Name
	   	lat, _ := strconv.ParseFloat(attachment.Latitude, 64)
	   	lng, _ := strconv.ParseFloat(attachment.Longitude, 64)
	   	latChar := 'N'
	   	if lat < 0 {
	   		latChar = 'S'
	   	}
	   	longChar := 'E'
	   	if lng < 0 {
	   		longChar = 'W'
	   	}
	   	formattedLoc := fmt.Sprintf("%.4f° %c %.4f° %c", math.Abs(lat), latChar, math.Abs(lng), longChar)

	   	content := &event.MessageEventContent{
	   		MsgType: event.MsgLocation,
	   		Body:    fmt.Sprintf("Location: %s\n%s", name, formattedLoc), //TODO link and stuff
	   		GeoURI:  fmt.Sprintf("geo:%.5f,%.5f", lat, lng),
	   	}

	   	return content, false, nil

	   case "reply":

	   	fmt.Printf("%+v\n", attachment)
	   	content := &event.MessageEventContent{
	   		Body:    message.Text,
	   		MsgType: event.MsgText,
	   	}
	   	portal.SetReply(content, attachment.ReplyID)
	   	return content, false, nil
	*/
}

func GetGroupmeFilename(attachmentUrlString string) string {
	attachmentUrl, _ := url.Parse(attachmentUrlString)
	urlParts := strings.Split(attachmentUrl.Path, ".")
	var fname1, fname2 string
	if len(urlParts) == 2 {
		fname1, fname2 = urlParts[1], urlParts[0]
	} else if len(urlParts) > 2 {
		fname1, fname2 = urlParts[2], urlParts[1]
	}
	return fmt.Sprintf("%s.%s", fname1, fname2)
}

func ErrorToNotice(err error, attachmentContainerType string) *bridgev2.ConvertedMessagePart {
	errMsg := "Failed to transfer attachment"
	if errors.Is(err, ErrURLNotFound) {
		errMsg = fmt.Sprintf("Unrecognized %s attachment type", attachmentContainerType)
	} else if errors.Is(err, ErrTooLargeFile) {
		errMsg = "Too large attachment"
	}
	return &bridgev2.ConvertedMessagePart{
		Type: event.EventMessage,
		Content: &event.MessageEventContent{
			MsgType: event.MsgNotice,
			Body:    errMsg,
		},
		Extra: map[string]any{
			"fi.mau.unsupported": true,
		},
	}
}
