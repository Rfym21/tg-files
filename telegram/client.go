package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type HTTPClient struct {
	botToken   string
	apiBaseURL string
	httpClient *http.Client
}

var _ Client = (*HTTPClient)(nil)

func NewHTTPClient(botToken, apiBaseURL string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &HTTPClient{
		botToken:   botToken,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *HTTPClient) Upload(ctx context.Context, request UploadRequest) (UploadedFile, error) {
	methodName, fieldName, err := uploadMethodAndField(request.Type)
	if err != nil {
		return UploadedFile{}, err
	}

	requestURL := c.methodURL(methodName)
	resp, err := c.doUploadRequest(ctx, requestURL, request, fieldName)
	if err != nil {
		return UploadedFile{}, err
	}

	var envelope uploadEnvelope
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return UploadedFile{}, fmt.Errorf("decode telegram upload response: %w", err)
	}
	if !envelope.OK {
		return UploadedFile{}, telegramAPIError(envelope.Description)
	}

	file, err := uploadedFileFromResult(request.Type, envelope.Result)
	if err != nil {
		return UploadedFile{}, err
	}
	if file.Type == "" {
		file.Type = request.Type
	}
	file.MessageID = envelope.Result.MessageID
	return file, nil
}

func (c *HTTPClient) Download(ctx context.Context, fileID string) (io.ReadCloser, error) {
	values := url.Values{}
	values.Set("file_id", fileID)

	resp, err := c.doJSONRequest(ctx, http.MethodPost, c.methodURL("getFile"), "application/x-www-form-urlencoded", func() (io.ReadCloser, string, error) {
		return io.NopCloser(strings.NewReader(values.Encode())), "application/x-www-form-urlencoded", nil
	})
	if err != nil {
		return nil, err
	}

	var envelope getFileEnvelope
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return nil, fmt.Errorf("decode telegram getFile response: %w", err)
	}
	if !envelope.OK {
		return nil, telegramAPIError(envelope.Description)
	}
	if envelope.Result.FilePath == "" {
		return nil, errors.New("telegram getFile response missing file_path")
	}

	return c.downloadFile(ctx, envelope.Result.FilePath)
}

func (c *HTTPClient) downloadFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	// Self-hosted Bot API Server (`--local`) returns an absolute filesystem path
	// in `file_path` instead of a relative URL path. When the host process can
	// see that path (typically via a shared volume), open it directly to skip
	// the redundant HTTP round-trip and avoid building a malformed download URL.
	if isLocalFilesystemPath(filePath) {
		if f, err := os.Open(filePath); err == nil {
			return f, nil
		} else if !os.IsNotExist(err) && !errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("open local telegram file: %w", err)
		}
		// Fall through to HTTP fetch when the local file isn't reachable
		// (e.g. running without the shared volume). The Bot API Server still
		// exposes /file/<token>/<absolute-path> so the request below can work.
	}

	fileURL := c.fileURL(filePath)
	var lastErr error

	for attempt := 0; attempt < 4; attempt++ {
		req, err := c.newRequestWithContext(ctx, http.MethodGet, fileURL, nil, "")
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = c.safeRequestError("telegram download failed", err)
		} else {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp.Body, nil
			}

			data, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read telegram download response: %w", readErr)
			}

			retry, delay, parseErr := shouldRetryTelegram(resp.StatusCode, data)
			if parseErr != nil {
				return nil, parseErr
			}
			if !retry {
				return nil, statusError(resp.StatusCode, data)
			}
			lastErr = statusError(resp.StatusCode, data)
			if err := sleepWithContext(ctx, backoffDelay(ctx, c.httpClient.Timeout, attempt, delay)); err != nil {
				return nil, err
			}
			continue
		}

		if attempt == 3 {
			break
		}
		if err := sleepWithContext(ctx, backoffDelay(ctx, c.httpClient.Timeout, attempt, 0)); err != nil {
			return nil, err
		}
	}

	if lastErr == nil {
		lastErr = errors.New("telegram download failed")
	}
	return nil, lastErr
}

func (c *HTTPClient) uploadBody(request UploadRequest, fieldName string) (io.ReadCloser, string) {
	pipeReader, pipeWriter := io.Pipe()
	// pipeReader must be closed on every path so the multipart writer goroutine can
	// observe the shutdown and exit instead of blocking on writes to the pipe.
	writer := multipart.NewWriter(pipeWriter)

	go func() {
		defer pipeWriter.Close()
		defer writer.Close()

		if err := writer.WriteField("chat_id", request.ChatID); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if request.Caption != "" {
			if err := writer.WriteField("caption", request.Caption); err != nil {
				_ = pipeWriter.CloseWithError(err)
				return
			}
		}

		part, err := createFilePart(writer, fieldName, request.Filename, request.MIMEType)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, request.Reader); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
	}()

	return pipeReader, writer.FormDataContentType()
}

func (c *HTTPClient) doUploadRequest(ctx context.Context, requestURL string, request UploadRequest, fieldName string) ([]byte, error) {
	var lastErr error
	readSeeker, replayable := request.Reader.(io.ReadSeeker)

	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			if !replayable {
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, errors.New("telegram upload cannot retry non-seekable reader")
			}
			if _, err := readSeeker.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("reset telegram upload reader: %w", err)
			}
		}

		attemptRequest := request
		attemptRequest.Reader = request.Reader
		if replayable {
			attemptRequest.Reader = readSeeker
		}
		reader, contentType := c.uploadBody(attemptRequest, fieldName)
		data, retry, delay, err := c.doSingleUploadAttempt(ctx, requestURL, reader, contentType)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !retry {
			return nil, err
		}
		lastErr = err
		if !replayable {
			return nil, safeUploadReplayError(err)
		}
		if attempt == 3 {
			break
		}
		if err := sleepWithContext(ctx, backoffDelay(ctx, c.httpClient.Timeout, attempt, delay)); err != nil {
			return nil, err
		}
	}

	if lastErr == nil {
		lastErr = errors.New("telegram upload failed")
	}
	return nil, lastErr
}

func (c *HTTPClient) doSingleUploadAttempt(ctx context.Context, requestURL string, reader io.ReadCloser, contentType string) ([]byte, bool, time.Duration, error) {
	defer reader.Close()

	req, err := c.newRequestWithContext(ctx, http.MethodPost, requestURL, reader, contentType)
	if err != nil {
		return nil, false, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, 0, ctx.Err()
		}
		return nil, true, 0, c.safeRequestError("telegram request failed", err)
	}

	data, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return nil, false, 0, fmt.Errorf("read telegram response: %w", readErr)
	}

	retry, delay, parseErr := shouldRetryTelegram(resp.StatusCode, data)
	if parseErr != nil {
		return nil, false, 0, parseErr
	}
	if !retry && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, false, 0, nil
	}
	return nil, retry, delay, statusError(resp.StatusCode, data)
}

func (c *HTTPClient) doJSONRequest(ctx context.Context, method, requestURL string, contentType string, bodyFactory func() (io.ReadCloser, string, error)) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < 4; attempt++ {
		reader, reqContentType, err := bodyFactory()
		if err != nil {
			return nil, err
		}

		req, err := c.newRequestWithContext(ctx, method, requestURL, reader, firstNonEmpty(reqContentType, contentType))
		if err != nil {
			reader.Close()
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			reader.Close()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = c.safeRequestError("telegram request failed", err)
		} else {
			data, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read telegram response: %w", readErr)
			}

			retry, delay, parseErr := shouldRetryTelegram(resp.StatusCode, data)
			if parseErr != nil {
				return nil, parseErr
			}
			if !retry && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return data, nil
			}
			if !retry {
				return nil, statusError(resp.StatusCode, data)
			}
			lastErr = statusError(resp.StatusCode, data)
			if err := sleepWithContext(ctx, backoffDelay(ctx, c.httpClient.Timeout, attempt, delay)); err != nil {
				return nil, err
			}
			continue
		}

		if attempt == 3 {
			break
		}
		if err := sleepWithContext(ctx, backoffDelay(ctx, c.httpClient.Timeout, attempt, 0)); err != nil {
			return nil, err
		}
	}

	if lastErr == nil {
		lastErr = errors.New("telegram request failed")
	}
	return nil, lastErr
}

func (c *HTTPClient) newRequestWithContext(ctx context.Context, method, requestURL string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, c.safeRequestError("create telegram request", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func safeUploadReplayError(cause error) error {
	if cause == nil {
		return errors.New("telegram upload cannot retry non-seekable reader")
	}
	return fmt.Errorf("telegram upload cannot safely retry non-seekable reader after failure: %w", cause)
}

func (c *HTTPClient) safeRequestError(message string, err error) error {
	return fmt.Errorf("%s: %s", message, c.sanitizeErrorText(err.Error()))
}

func (c *HTTPClient) sanitizeErrorText(text string) string {
	if text == "" {
		return text
	}
	if c.botToken != "" {
		text = strings.ReplaceAll(text, c.botToken, "[redacted]")
	}
	text = strings.ReplaceAll(text, "/bot[redacted]/", "/bot<TOKEN>/")
	text = strings.ReplaceAll(text, "/file/bot[redacted]/", "/file/bot<TOKEN>/")
	return text
}

func createFilePart(writer *multipart.Writer, fieldName, filename, mimeType string) (io.Writer, error) {
	header := textproto.MIMEHeader{}
	contentDisposition := fmt.Sprintf(`form-data; name=%q`, fieldName)
	if filename != "" {
		contentDisposition += fmt.Sprintf(`; filename=%q`, filename)
	}
	header.Set("Content-Disposition", contentDisposition)
	if mimeType != "" {
		header.Set("Content-Type", mimeType)
	}
	return writer.CreatePart(header)
}

func uploadMethodAndField(fileType string) (string, string, error) {
	switch fileType {
	case TypePhoto:
		return "sendPhoto", "photo", nil
	case TypeVideo:
		return "sendVideo", "video", nil
	case TypeAudio:
		return "sendAudio", "audio", nil
	case TypeAnimation:
		return "sendAnimation", "animation", nil
	case TypeDocument:
		return "sendDocument", "document", nil
	default:
		return "", "", fmt.Errorf("unsupported telegram upload type %q", fileType)
	}
}

func (c *HTTPClient) methodURL(method string) string {
	return c.apiBaseURL + "/bot" + c.botToken + "/" + method
}

func (c *HTTPClient) fileURL(filePath string) string {
	return c.apiBaseURL + "/file/bot" + c.botToken + "/" + strings.TrimLeft(path.Clean(filePath), "/")
}

// isLocalFilesystemPath reports whether file_path looks like an absolute path
// from a self-hosted Bot API Server (e.g. /var/lib/telegram-bot-api/...).
func isLocalFilesystemPath(filePath string) bool {
	if filePath == "" {
		return false
	}
	if strings.HasPrefix(filePath, "/") {
		return true
	}
	// Windows-style drive letter, just in case.
	if len(filePath) >= 3 && filePath[1] == ':' && (filePath[2] == '/' || filePath[2] == '\\') {
		return true
	}
	return false
}

func shouldRetryTelegram(statusCode int, data []byte) (bool, time.Duration, error) {
	if statusCode == http.StatusTooManyRequests || statusCode >= 500 {
		var envelope telegramErrorEnvelope
		if len(data) > 0 && json.Unmarshal(data, &envelope) == nil {
			if envelope.Parameters.RetryAfter > 0 {
				return true, time.Duration(envelope.Parameters.RetryAfter) * time.Second, nil
			}
		}
		return true, 0, nil
	}
	return false, 0, nil
}

func backoffDelay(ctx context.Context, clientTimeout time.Duration, attempt int, preferred time.Duration) time.Duration {
	if preferred > 0 {
		return cappedRetryAfter(ctx, clientTimeout, preferred)
	}
	delay := 25 * time.Millisecond * time.Duration(1<<attempt)
	if delay > 200*time.Millisecond {
		return 200 * time.Millisecond
	}
	return delay
}

func cappedRetryAfter(ctx context.Context, clientTimeout, retryAfter time.Duration) time.Duration {
	if retryAfter <= 0 {
		return 0
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0
		}
		if retryAfter > remaining {
			return remaining
		}
		return retryAfter
	}
	if clientTimeout > 0 && retryAfter > clientTimeout {
		return clientTimeout
	}
	return retryAfter
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
		<-ctx.Done()
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}
}

func statusError(statusCode int, data []byte) error {
	if len(data) > 0 {
		var envelope telegramErrorEnvelope
		if json.Unmarshal(data, &envelope) == nil && envelope.Description != "" {
			return telegramAPIError(envelope.Description)
		}
	}
	return fmt.Errorf("telegram request failed with status %d", statusCode)
}

func telegramAPIError(description string) error {
	description = strings.TrimSpace(description)
	if description == "" {
		description = "telegram api error"
	}
	return errors.New(description)
}

const (
	telegramTypeVoice     = "voice"
	telegramTypeVideoNote = "video_note"
)

func uploadedFileFromResult(fileType string, result uploadResult) (UploadedFile, error) {
	candidates, err := uploadResultCandidateTypes(fileType)
	if err != nil {
		return UploadedFile{}, err
	}
	for _, candidate := range candidates {
		if file, ok := uploadedFileByType(candidate, result); ok {
			return file, nil
		}
	}
	return UploadedFile{}, fmt.Errorf("telegram upload response missing %s", fileType)
}

func uploadResultCandidateTypes(fileType string) ([]string, error) {
	switch fileType {
	case TypePhoto:
		return []string{TypePhoto, TypeDocument, TypeAnimation, TypeVideo, TypeAudio, telegramTypeVoice, telegramTypeVideoNote}, nil
	case TypeVideo:
		return []string{TypeVideo, TypeDocument, telegramTypeVideoNote, TypeAnimation, TypeAudio, telegramTypeVoice, TypePhoto}, nil
	case TypeAudio:
		return []string{TypeAudio, TypeDocument, telegramTypeVoice, TypeVideo, TypeAnimation, telegramTypeVideoNote, TypePhoto}, nil
	case TypeAnimation:
		return []string{TypeAnimation, TypeDocument, TypeVideo, telegramTypeVideoNote, TypePhoto, TypeAudio, telegramTypeVoice}, nil
	case TypeDocument:
		return []string{TypeDocument, TypeVideo, TypeAudio, TypeAnimation, TypePhoto, telegramTypeVoice, telegramTypeVideoNote}, nil
	default:
		return nil, fmt.Errorf("unsupported telegram upload type %q", fileType)
	}
}

func uploadedFileByType(fileType string, result uploadResult) (UploadedFile, bool) {
	switch fileType {
	case TypePhoto:
		return photoToUploadedFile(result.Photo)
	case TypeVideo:
		return mediaToUploadedFile(TypeVideo, result.Video)
	case TypeAudio:
		return mediaToUploadedFile(TypeAudio, result.Audio)
	case TypeAnimation:
		return mediaToUploadedFile(TypeAnimation, result.Animation)
	case TypeDocument:
		return mediaToUploadedFile(TypeDocument, result.Document)
	case telegramTypeVoice:
		return mediaToUploadedFile(telegramTypeVoice, result.Voice)
	case telegramTypeVideoNote:
		return mediaToUploadedFile(telegramTypeVideoNote, result.VideoNote)
	default:
		return UploadedFile{}, false
	}
}

func photoToUploadedFile(photos []telegramFile) (UploadedFile, bool) {
	if len(photos) == 0 {
		return UploadedFile{}, false
	}
	largest := photos[0]
	for _, photo := range photos[1:] {
		if photo.FileSize >= largest.FileSize {
			largest = photo
		}
	}
	return UploadedFile{
		Type:         TypePhoto,
		FileID:       largest.FileID,
		FileUniqueID: largest.FileUniqueID,
		FileSize:     largest.FileSize,
		MIMEType:     largest.MIMEType,
	}, true
}

func mediaToUploadedFile(fileType string, media telegramFile) (UploadedFile, bool) {
	if media.FileID == "" {
		return UploadedFile{}, false
	}
	return UploadedFile{
		Type:         fileType,
		FileID:       media.FileID,
		FileUniqueID: media.FileUniqueID,
		FileSize:     media.FileSize,
		MIMEType:     media.MIMEType,
	}, true
}

type uploadEnvelope struct {
	OK          bool         `json:"ok"`
	Description string       `json:"description"`
	Result      uploadResult `json:"result"`
}

type uploadResult struct {
	MessageID int64          `json:"message_id"`
	Document  telegramFile   `json:"document"`
	Video     telegramFile   `json:"video"`
	Audio     telegramFile   `json:"audio"`
	Animation telegramFile   `json:"animation"`
	Photo     []telegramFile `json:"photo"`
	Voice     telegramFile   `json:"voice"`
	VideoNote telegramFile   `json:"video_note"`
}

type telegramFile struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int64  `json:"file_size"`
	MIMEType     string `json:"mime_type"`
}

type getFileEnvelope struct {
	OK          bool          `json:"ok"`
	Description string        `json:"description"`
	Result      getFileResult `json:"result"`
}

type getFileResult struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

type telegramErrorEnvelope struct {
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}
