package b2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeB2Server mimics the B2 native API surface the client uses, so the tests
// stay offline and deterministic.
func fakeB2Server(t *testing.T, arquivos []map[string]any) *httptest.Server {
	t.Helper()
	var servidor *httptest.Server
	mux := http.NewServeMux()

	mux.HandleFunc("/b2api/v2/b2_authorize_account", func(w http.ResponseWriter, r *http.Request) {
		usuario, senha, ok := r.BasicAuth()
		if !ok || usuario != "key-id" || senha != "key-secret" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":          "acc-1",
			"authorizationToken": "token-1",
			"apiUrl":             servidor.URL,
		})
	})
	mux.HandleFunc("/b2api/v2/b2_list_buckets", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token-1" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"buckets": []map[string]any{{"bucketId": "bucket-1", "bucketName": "ebooks"}},
		})
	})
	mux.HandleFunc("/b2api/v2/b2_list_file_names", func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		var pedido map[string]any
		_ = json.Unmarshal(corpo, &pedido)
		inicio, _ := pedido["startFileName"].(string)
		pagina := arquivos
		if inicio != "" {
			pagina = nil
			for _, arquivo := range arquivos {
				if nome, _ := arquivo["fileName"].(string); nome > inicio {
					pagina = append(pagina, arquivo)
				}
			}
		}
		resposta := map[string]any{"files": pagina}
		if len(pagina) > 2 {
			resposta["files"] = pagina[:2]
			resposta["nextFileName"] = pagina[1]["fileName"]
		}
		_ = json.NewEncoder(w).Encode(resposta)
	})
	mux.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/epub+zip")
		_, _ = w.Write([]byte("conteudo-do-livro"))
	})

	servidor = httptest.NewServer(mux)
	t.Cleanup(servidor.Close)
	return servidor
}

func newTestClient(t *testing.T, servidor *httptest.Server) *Client {
	t.Helper()
	return NewWithEndpoint("key-id", "key-secret", servidor.URL, servidor.Client())
}

func TestClient_ListPaginates(t *testing.T) {
	servidor := fakeB2Server(t, []map[string]any{
		{"fileName": "Cat/a.pdf", "contentLength": 1, "contentSha1": "s1", "uploadTimestamp": 1},
		{"fileName": "Cat/b.epub", "contentLength": 2, "contentSha1": "s2", "uploadTimestamp": 2},
		{"fileName": "Cat/c.pdf", "contentLength": 3, "contentSha1": "s3", "uploadTimestamp": 3},
	})

	cliente := newTestClient(t, servidor)
	files, err := cliente.List(context.Background(), "ebooks")
	if err != nil {
		t.Fatalf("List erro = %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("List devolveu %d arquivos, want 3 (paginação)", len(files))
	}
	if files[1].Name != "Cat/b.epub" || files[1].Size != 2 || files[1].SHA1 != "s2" {
		t.Errorf("files[1] = %+v", files[1])
	}
}

func TestClient_OpenStreamsFile(t *testing.T) {
	servidor := fakeB2Server(t, nil)
	cliente := newTestClient(t, servidor)

	corpo, err := cliente.Open(context.Background(), "ebooks", "Cat/livro epub.epub")
	if err != nil {
		t.Fatalf("Open erro = %v", err)
	}
	defer func() { _ = corpo.Close() }()
	conteudo, _ := io.ReadAll(corpo)
	if string(conteudo) != "conteudo-do-livro" {
		t.Errorf("conteúdo = %q", string(conteudo))
	}
}

func TestClient_ListWithoutBucketFails(t *testing.T) {
	servidor := fakeB2Server(t, nil)
	cliente := newTestClient(t, servidor)

	_, err := cliente.List(context.Background(), "inexistente")
	if err == nil {
		t.Fatal("List deveria falhar para bucket ausente")
	}
	if !strings.Contains(err.Error(), "inexistente") {
		t.Errorf("erro = %v, want menção ao bucket recebido", err)
	}
}

func TestClient_ReauthorizesOnceOn401(t *testing.T) {
	tentativas := 0
	mux := http.NewServeMux()
	var servidor *httptest.Server
	mux.HandleFunc("/b2api/v2/b2_authorize_account", func(w http.ResponseWriter, r *http.Request) {
		usuario, senha, _ := r.BasicAuth()
		if usuario != "key-id" || senha != "key-secret" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":          "acc-1",
			"authorizationToken": fmt.Sprintf("token-%d", tentativas),
			"apiUrl":             servidor.URL,
		})
	})
	mux.HandleFunc("/b2api/v2/b2_list_buckets", func(w http.ResponseWriter, r *http.Request) {
		tentativas++
		if r.Header.Get("Authorization") == "token-0" {
			http.Error(w, `{"code":"expired_auth_token"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"buckets": []map[string]any{{"bucketId": "b", "bucketName": "ebooks"}},
		})
	})
	mux.HandleFunc("/b2api/v2/b2_list_file_names", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{}})
	})
	servidor = httptest.NewServer(mux)
	defer servidor.Close()

	cliente := NewWithEndpoint("key-id", "key-secret", servidor.URL, servidor.Client())
	if _, err := cliente.List(context.Background(), "ebooks"); err != nil {
		t.Fatalf("List erro = %v (deveria reautenticar no 401)", err)
	}
}

func TestClient_BasicAuthHeader(t *testing.T) {
	// Guard against regressions in the credential encoding itself.
	esperado := "Basic " + base64.StdEncoding.EncodeToString([]byte("key-id:key-secret"))
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != esperado {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), esperado)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":          "acc-1",
			"authorizationToken": "t",
			"apiUrl":             "http://example.invalid",
		})
	}))
	defer servidor.Close()

	cliente := NewWithEndpoint("key-id", "key-secret", servidor.URL, servidor.Client())
	if _, err := cliente.List(context.Background(), "ebooks"); err == nil {
		t.Fatal("esperava erro ao falar com apiUrl inválida depois do authorize")
	}
}
