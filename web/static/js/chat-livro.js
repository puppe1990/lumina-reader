/* "Conversar com o livro": manda a pergunta + o trecho atual (capítulo do EPUB
   ou página do PDF) para o provedor de LLM e mostra a resposta em bolhas. */
(function () {
  "use strict";

  var raiz = document.querySelector("[data-chat-root]");
  if (!raiz) return;

  var livroId = raiz.getAttribute("data-book");
  var lista = raiz.querySelector("[data-chat-mensagens]");
  var campo = raiz.querySelector("[data-chat-pergunta]");
  var form = raiz.querySelector("[data-chat-form]");
  var rotuloPosicao = raiz.querySelector("[data-chat-posicao]");
  var envio = raiz.querySelector("[data-chat-enviar]");
  var sugestoes = raiz.querySelector("[data-chat-sugestoes]");
  var historico = [];
  var enviando = false;

  var SUGESTOES = ["Resuma o livro", "Qual é a ideia central?", "O que o autor diz sobre "];

  function token() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
  }

  function bolha(papel, texto) {
    var div = document.createElement("div");
    div.className =
      papel === "user"
        ? "max-w-[85%] self-end rounded-2xl rounded-br-md bg-primary px-3.5 py-2 text-[13px] leading-5 text-on-primary shadow-sm"
        : "max-w-[92%] self-start rounded-2xl rounded-bl-md bg-surface-container-low px-3.5 py-2 text-[13px] leading-5 text-on-surface shadow-sm";
    div.textContent = texto;
    lista.appendChild(div);
    lista.scrollTop = lista.scrollHeight;
    return div;
  }

  function aviso(texto) {
    var div = document.createElement("div");
    div.className =
      "max-w-[92%] self-start rounded-2xl border border-outline-variant/60 bg-surface-container px-3.5 py-2 text-[12px] leading-5 text-on-surface-variant";
    div.textContent = texto;
    lista.appendChild(div);
    lista.scrollTop = lista.scrollHeight;
  }

  function pensando() {
    var div = document.createElement("div");
    div.className =
      "flex items-center gap-2 self-start rounded-2xl bg-surface-container-low px-3.5 py-2 text-[12px] text-secondary shadow-sm";
    div.innerHTML =
      '<span class="material-symbols-outlined animate-spin text-[14px] text-primary">progress_activity</span>lendo o trecho…';
    lista.appendChild(div);
    lista.scrollTop = lista.scrollHeight;
    return div;
  }

  // Trecho atual: o PDF expõe página + texto via window.luminaPdf; o EPUB usa
  // o capítulo que está na tela.
  function contextoEpub() {
    var artigo = document.querySelector(".leitor-texto") || document.querySelector("#readerCanvas");
    return { posicao: rotuloPosicao ? rotuloPosicao.textContent.trim() : "", texto: artigo ? artigo.innerText : "" };
  }

  async function contexto() {
    try {
      if (window.luminaPdf && typeof window.luminaPdf.contexto === "function") {
        return await window.luminaPdf.contexto();
      }
    } catch (e) {}
    return contextoEpub();
  }

  async function atualizarPosicao() {
    if (!rotuloPosicao) return;
    try {
      if (window.luminaPdf && typeof window.luminaPdf.posicao === "function") {
        rotuloPosicao.textContent = window.luminaPdf.posicao();
      }
    } catch (e) {}
  }

  function montarSugestoes() {
    sugestoes.innerHTML = "";
    SUGESTOES.forEach(function (texto) {
      var botao = document.createElement("button");
      botao.type = "button";
      botao.className =
        "rounded-full border border-outline-variant/60 px-2.5 py-1 text-[11px] text-on-surface-variant transition-colors hover:border-primary hover:text-primary";
      botao.textContent = texto;
      botao.addEventListener("click", function () {
        campo.value = texto;
        campo.focus();
      });
      sugestoes.appendChild(botao);
    });
  }

  function travar(estado) {
    enviando = estado;
    campo.disabled = estado;
    envio.disabled = estado;
    envio.classList.toggle("opacity-60", estado);
  }

  // O proxy na frente do app corta requisições longas: o POST devolve um id na
  // hora e a resposta chega pelo polling até o trabalho terminar.
  function aguardar(id) {
    var inicio = Date.now();
    return new Promise(function (resolve) {
      async function passo() {
        if (Date.now() - inicio > 180000) {
          resolve({ erro: "O provedor demorou demais para responder; tente de novo." });
          return;
        }
        try {
          var resposta = await fetch("/livros/" + livroId + "/chat/" + encodeURIComponent(id));
          var dados = await resposta.json().catch(function () {
            return {};
          });
          if (dados.estado && dados.estado !== "rodando") {
            resolve(dados);
            return;
          }
          if (!resposta.ok) {
            resolve({ erro: dados.erro || "Perdi a pergunta no caminho; tente de novo." });
            return;
          }
        } catch (e) {}
        setTimeout(passo, 1400);
      }
      passo();
    });
  }

  async function perguntar(texto) {
    var pergunta = texto.trim();
    if (!pergunta || enviando) return;
    bolha("user", pergunta);
    campo.value = "";
    travar(true);
    var carregando = pensando();
    var ctx = await contexto();
    try {
      var resposta = await fetch("/livros/" + livroId + "/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": token() },
        body: JSON.stringify({
          pergunta: pergunta,
          contexto: ctx.texto || "",
          posicao: ctx.posicao || "",
          historico: JSON.stringify(historico),
        }),
      });
      var dados = await resposta.json().catch(function () {
        return {};
      });
      if (!resposta.ok || !dados.id) {
        carregando.remove();
        aviso(dados.erro || "Não consegui perguntar agora. Tente de novo.");
        return;
      }
      var final = await aguardar(dados.id);
      carregando.remove();
      if (final.estado === "pronto" && final.resposta) {
        bolha("assistant", final.resposta);
        historico.push({ papel: "user", texto: pergunta });
        historico.push({ papel: "assistant", texto: final.resposta });
        if (historico.length > 8) historico = historico.slice(-8);
      } else {
        aviso(final.erro || "O provedor não respondeu; tente de novo.");
      }
    } catch (erro) {
      carregando.remove();
      aviso("Não consegui falar com o servidor: " + (erro && erro.message ? erro.message : "erro de rede"));
    } finally {
      travar(false);
      campo.focus();
    }
  }

  form.addEventListener("submit", function (ev) {
    ev.preventDefault();
    perguntar(campo.value);
  });
  campo.addEventListener("keydown", function (ev) {
    if (ev.key === "Enter" && !ev.shiftKey) {
      ev.preventDefault();
      perguntar(campo.value);
    }
  });

  // posição viva (o PDF muda de página) sem depender de evento do diálogo
  new MutationObserver(function () {
    if (raiz.open) atualizarPosicao();
  }).observe(raiz, { attributes: true, attributeFilter: ["open"] });

  montarSugestoes();
  aviso("Pergunte o que quiser sobre o livro inteiro — respondo com base no texto dele.");
})();
