// Package b2 fala com a API nativa do Backblaze B2: autorização, listagem do
// balde e download por nome. O token em memória vale 24h; um 401 reautoriza uma
// única vez antes de devolver o erro.
package b2

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/puppe1990/lumina-reader/internal/library"
)

// endpoint é a base da API; authorize pendura /b2api/v2/b2_authorize_account.
const defaultEndpoint = "https://api.backblazeb2.com"

const authorizePath = "/b2api/v2/b2_authorize_account"

type FileInfo struct {
	Name            string
	Size            int64
	SHA1            string
	UploadTimestamp int64
}

type Client struct {
	keyID    string
	key      string
	endpoint string
	http     *http.Client

	mu        sync.Mutex
	token     string
	apiURL    string
	accountID string
}

func New(keyID, key string) *Client {
	return NewWithEndpoint(keyID, key, defaultEndpoint, &http.Client{Timeout: 120 * time.Second})
}

func NewWithEndpoint(keyID, key, endpoint string, httpClient *http.Client) *Client {
	return &Client{keyID: keyID, key: key, endpoint: endpoint, http: httpClient}
}

// List devolve todos os arquivos do balde, paginando b2_list_file_names.
func (c *Client) List(ctx context.Context, bucketName string) ([]FileInfo, error) {
	bucketID, err := c.bucketID(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	var arquivos []FileInfo
	start := ""
	for {
		payload := map[string]any{"bucketId": bucketID, "maxFileCount": 10000}
		if start != "" {
			payload["startFileName"] = start
		}
		var pagina struct {
			Files []struct {
				FileName        string `json:"fileName"`
				ContentLength   int64  `json:"contentLength"`
				ContentSHA1     string `json:"contentSha1"`
				UploadTimestamp int64  `json:"uploadTimestamp"`
			} `json:"files"`
			NextFileName *string `json:"nextFileName"`
		}
		if err := c.post(ctx, "b2_list_file_names", payload, &pagina); err != nil {
			return nil, fmt.Errorf("listar arquivos de %q: %w", bucketName, err)
		}
		for _, file := range pagina.Files {
			arquivos = append(arquivos, FileInfo{
				Name:            file.FileName,
				Size:            file.ContentLength,
				SHA1:            file.ContentSHA1,
				UploadTimestamp: file.UploadTimestamp,
			})
		}
		if pagina.NextFileName == nil || *pagina.NextFileName == "" {
			return arquivos, nil
		}
		start = *pagina.NextFileName
	}
}

// ListRemote adapta List para a interface de sincronização da estante.
func (c *Client) ListRemote(ctx context.Context, bucketName string) ([]library.RemoteFile, error) {
	files, err := c.List(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	remotos := make([]library.RemoteFile, 0, len(files))
	for _, file := range files {
		remotos = append(remotos, library.RemoteFile{Key: file.Name, Size: file.Size, SHA1: file.SHA1})
	}
	return remotos, nil
}

// Open baixa o arquivo pelo nome, pronto para streaming.
func (c *Client) Open(ctx context.Context, bucketName, fileName string) (io.ReadCloser, error) {
	apiURL, err := c.ensureToken(ctx)
	if err != nil {
		return nil, err
	}
	destino := apiURL + "/file/" + bucketName + "/" + escapeFilePath(fileName)
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, destino, nil)
	if err != nil {
		return nil, fmt.Errorf("montar download de %q: %w", fileName, err)
	}
	resposta, err := c.doAuthorized(pedido)
	if err != nil {
		return nil, fmt.Errorf("baixar %q: %w", fileName, err)
	}
	if resposta.StatusCode != http.StatusOK {
		defer func() { _ = resposta.Body.Close() }()
		return nil, apiError(fmt.Sprintf("baixar %q", fileName), resposta)
	}
	return resposta.Body, nil
}

func (c *Client) bucketID(ctx context.Context, bucketName string) (string, error) {
	if _, err := c.ensureToken(ctx); err != nil {
		return "", err
	}
	c.mu.Lock()
	accountID := c.accountID
	c.mu.Unlock()

	var resposta struct {
		Buckets []struct {
			BucketID   string `json:"bucketId"`
			BucketName string `json:"bucketName"`
		} `json:"buckets"`
	}
	if err := c.post(ctx, "b2_list_buckets", map[string]any{"accountId": accountID, "bucketName": bucketName}, &resposta); err != nil {
		return "", fmt.Errorf("listar baldes: %w", err)
	}
	for _, bucket := range resposta.Buckets {
		if bucket.BucketName == bucketName {
			return bucket.BucketID, nil
		}
	}
	return "", fmt.Errorf("bucket %q não encontrado na conta B2", bucketName)
}

func (c *Client) post(ctx context.Context, operacao string, payload map[string]any, resultado any) error {
	apiURL, err := c.ensureToken(ctx)
	if err != nil {
		return err
	}
	corpo, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializar %s: %w", operacao, err)
	}
	pedido, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/b2api/v2/"+operacao, bytes.NewReader(corpo))
	if err != nil {
		return fmt.Errorf("montar %s: %w", operacao, err)
	}
	pedido.Header.Set("Content-Type", "application/json")

	resposta, err := c.doAuthorized(pedido)
	if err != nil {
		return err
	}
	defer func() { _ = resposta.Body.Close() }()
	if resposta.StatusCode != http.StatusOK {
		return apiError(operacao, resposta)
	}
	return json.NewDecoder(resposta.Body).Decode(resultado)
}

// doAuthorized repete uma vez depois de reautorizar quando o B2 responde 401
// (token expirado) — o corpo é reconstruído via GetBody para não reenviar vazio.
func (c *Client) doAuthorized(pedido *http.Request) (*http.Response, error) {
	resposta, err := c.send(pedido)
	if err != nil {
		return nil, err
	}
	if resposta.StatusCode != http.StatusUnauthorized {
		return resposta, nil
	}
	_ = resposta.Body.Close()
	if err := c.authorize(pedido.Context()); err != nil {
		return nil, err
	}

	reenvio := pedido.Clone(pedido.Context())
	if pedido.GetBody != nil {
		corpo, err := pedido.GetBody()
		if err != nil {
			return nil, fmt.Errorf("reconstruir corpo para reenvio: %w", err)
		}
		reenvio.Body = corpo
	}
	return c.send(reenvio)
}

func (c *Client) send(pedido *http.Request) (*http.Response, error) {
	clone := pedido.Clone(pedido.Context())
	c.mu.Lock()
	clone.Header.Set("Authorization", c.token)
	c.mu.Unlock()

	resposta, err := c.http.Do(clone)
	if err != nil {
		return nil, fmt.Errorf("requisição B2: %w", err)
	}
	return resposta, nil
}

// ensureToken autoriza na primeira chamada e devolve o apiUrl vigente.
func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	apiURL, temToken := c.apiURL, c.token != ""
	c.mu.Unlock()
	if temToken {
		return apiURL, nil
	}
	if err := c.authorize(ctx); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.apiURL, nil
}

func (c *Client) authorize(ctx context.Context) error {
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+authorizePath, nil)
	if err != nil {
		return err
	}
	credencial := base64.StdEncoding.EncodeToString([]byte(c.keyID + ":" + c.key))
	pedido.Header.Set("Authorization", "Basic "+credencial)

	resposta, err := c.http.Do(pedido)
	if err != nil {
		return fmt.Errorf("autorizar no B2: %w", err)
	}
	defer func() { _ = resposta.Body.Close() }()
	if resposta.StatusCode != http.StatusOK {
		return apiError("autorizar", resposta)
	}

	var dados struct {
		AccountID          string `json:"accountId"`
		AuthorizationToken string `json:"authorizationToken"`
		APIURL             string `json:"apiUrl"`
	}
	if err := json.NewDecoder(resposta.Body).Decode(&dados); err != nil {
		return fmt.Errorf("decodificar autorização B2: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = dados.AuthorizationToken
	c.apiURL = dados.APIURL
	c.accountID = dados.AccountID
	return nil
}

func apiError(operacao string, resposta *http.Response) error {
	corpo, _ := io.ReadAll(io.LimitReader(resposta.Body, 400))
	return fmt.Errorf("%s: B2 respondeu %d: %s", operacao, resposta.StatusCode, string(corpo))
}

// escapeFilePath codifica cada segmento do caminho e preserva as barras — o B2
// aceita o nome codificado por inteiro, mas %2F em excesso confunde proxies.
func escapeFilePath(nome string) string {
	partes := splitSlash(nome)
	for i, parte := range partes {
		partes[i] = url.PathEscape(parte)
	}
	return joinSlash(partes)
}

func splitSlash(s string) []string {
	var partes []string
	atual := ""
	for _, r := range s {
		if r == '/' {
			partes = append(partes, atual)
			atual = ""
			continue
		}
		atual += string(r)
	}
	return append(partes, atual)
}

func joinSlash(partes []string) string {
	out := ""
	for i, parte := range partes {
		if i > 0 {
			out += "/"
		}
		out += parte
	}
	return out
}
