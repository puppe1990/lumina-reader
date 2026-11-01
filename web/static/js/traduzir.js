/* Tradução da seleção no próprio navegador: o Translator embutido do Chrome
   (modelo local, sem servidor e sem LLM) traduz a palavra/trecho selecionado.
   Resultado fica em localStorage — repetir é instantâneo. */
(function () {
  "use strict";

  var areaPdf = document.getElementById("pdf-paginas");
  var botao = null;
  var cartao = null;
  var origem = null;
  var tradutor = null;
  var tradutorIdioma = "";
  var detector = null;
  var CHAVE_CACHE = "lumina-traducao:";

  function suportado() {
    return typeof window.Translator !== "undefined";
  }

  function lerCache(texto) {
    try {
      return localStorage.getItem(CHAVE_CACHE + texto);
    } catch (e) {
      return null;
    }
  }

  function gravarCache(texto, traducao) {
    try {
      localStorage.setItem(CHAVE_CACHE + texto, traducao);
    } catch (e) {}
  }

  function esconderBotao() {
    if (botao) botao.hidden = true;
  }

  function fecharCartao() {
    if (cartao) {
      cartao.remove();
      cartao = null;
    }
  }

  function garantirBotao() {
    if (botao) return botao;
    botao = document.createElement("button");
    botao.type = "button";
    botao.hidden = true;
    botao.textContent = "Traduzir";
    botao.className =
      "fixed z-[70] flex items-center gap-1.5 rounded-full bg-inverse-surface px-3 py-1.5 text-[12px] font-medium text-inverse-on-surface shadow-lg";
    botao.addEventListener("mousedown", function (ev) {
      ev.preventDefault(); // não perde a seleção no clique
    });
    botao.addEventListener("click", function () {
      var texto = origem && origem.texto;
      esconderBotao();
      if (texto) traduzir(texto, origem.rect);
    });
    document.body.appendChild(botao);
    return botao;
  }

  function posicionar(alvo, rect) {
    var largura = alvo.offsetWidth || 80;
    var esquerda = Math.min(Math.max(rect.left + rect.width / 2 - largura / 2, 8), window.innerWidth - largura - 8);
    var topo = rect.bottom + 8;
    if (topo + 40 > window.innerHeight) topo = Math.max(8, rect.top - 40);
    alvo.style.left = esquerda + "px";
    alvo.style.top = topo + "px";
  }

  function mostrarBotao(texto, rect) {
    if (!texto) {
      esconderBotao();
      return;
    }
    var el = garantirBotao();
    origem = { texto: texto, rect: rect };
    posicionar(el, rect);
    el.hidden = false;
  }

  function cartaoBase(ancora) {
    fecharCartao();
    var div = document.createElement("div");
    div.className =
      "fixed z-[70] w-[min(92vw,22rem)] rounded-2xl border border-outline-variant/50 bg-surface p-3 text-on-surface shadow-xl";
    document.body.appendChild(div);
    cartao = div;
    if (ancora) posicionar(div, ancora);
    return div;
  }

  function linhaDeAcoes(container, traducao) {
    var linha = document.createElement("div");
    linha.className = "mt-2 flex items-center justify-between text-[11px] text-secondary";
    var copiar = document.createElement("button");
    copiar.type = "button";
    copiar.className = "rounded-full px-2 py-1 hover:bg-surface-container";
    copiar.textContent = "Copiar";
    copiar.addEventListener("click", function () {
      try {
        navigator.clipboard.writeText(traducao);
        copiar.textContent = "Copiado";
      } catch (e) {}
    });
    var fechar = document.createElement("button");
    fechar.type = "button";
    fechar.className = "rounded-full px-2 py-1 hover:bg-surface-container";
    fechar.textContent = "Fechar";
    fechar.addEventListener("click", fecharCartao);
    linha.appendChild(copiar);
    linha.appendChild(fechar);
    container.appendChild(linha);
  }

  function aviso(container, texto) {
    var p = document.createElement("p");
    p.className = "text-[12px] leading-5 text-on-surface";
    p.textContent = texto;
    container.appendChild(p);
  }

  // A API exige o par de idiomas; detectamos o idioma do trecho e caímos em
  // inglês quando o detector não ajuda.
  async function idiomaDoTexto(texto) {
    try {
      if (!detector && typeof window.LanguageDetector !== "undefined") {
        detector = await window.LanguageDetector.create();
      }
      if (detector) {
        var achados = await detector.detect(texto);
        if (achados && achados[0] && achados[0].detectedLanguage) return achados[0].detectedLanguage;
      }
    } catch (e) {}
    return "en";
  }

  async function disponibilidadeDoPar(idioma) {
    try {
      return await window.Translator.availability({ sourceLanguage: idioma, targetLanguage: "pt" });
    } catch (e) {
      return "unavailable";
    }
  }

  async function obterTradutor(idioma, aoProgredir) {
    if (tradutor && tradutorIdioma === idioma) return tradutor;
    var monitor = function (m) {
      m.addEventListener("downloadprogress", function (evento) {
        if (aoProgredir) aoProgredir(Math.round((evento.loaded || 0) * 100));
      });
    };
    try {
      tradutor = await window.Translator.create({
        sourceLanguage: idioma,
        targetLanguage: "pt",
        monitor: monitor,
      });
      tradutorIdioma = idioma;
    } catch (erroCriacao) {
      if (idioma === "en") throw erroCriacao;
      tradutor = await window.Translator.create({ sourceLanguage: "en", targetLanguage: "pt", monitor: monitor });
      tradutorIdioma = "en";
    }
    return tradutor;
  }

  async function traduzir(texto, rect) {
    if (!texto) return;
    var div = cartaoBase(rect);
    var fonte = document.createElement("p");
    fonte.className = "mb-2 max-h-16 overflow-hidden text-[11px] italic leading-4 text-secondary";
    fonte.textContent = texto.length > 220 ? texto.slice(0, 220) + "…" : texto;
    div.appendChild(fonte);

    if (!suportado()) {
      aviso(div, "Este navegador não tem o tradutor offline. Chrome, Edge ou Brave atualizados traduzem sem servidor.");
      return;
    }

    var emCache = lerCache(texto);
    if (emCache) {
      aviso(div, emCache);
      linhaDeAcoes(div, emCache);
      return;
    }

    var status = document.createElement("p");
    status.className = "flex items-center gap-2 text-[13px] leading-5";
    var icone = document.createElement("span");
    icone.className = "material-symbols-outlined animate-spin text-[16px] text-primary";
    icone.textContent = "progress_activity";
    status.appendChild(icone);
    status.appendChild(document.createTextNode("traduzindo…"));
    div.appendChild(status);

    try {
      var idioma = await idiomaDoTexto(texto);
      if (idioma && idioma.toLowerCase().startsWith("pt")) {
        status.textContent = "O trecho já está em português.";
        return;
      }
      var disponibilidade = await disponibilidadeDoPar(idioma);
      if (disponibilidade === "unavailable" && idioma !== "en") {
        // detecção fraca (ex.: "und") ou idioma sem modelo: o acervo é
        // majoritariamente inglês, então tentamos en → pt.
        idioma = "en";
        disponibilidade = await disponibilidadeDoPar(idioma);
      }
      if (disponibilidade === "unavailable") {
        status.textContent = "O tradutor offline não tem o par " + idioma + " → português.";
        return;
      }
      var rotulo = status.lastChild;
      if (disponibilidade === "downloadable" || disponibilidade === "downloading") {
        rotulo.textContent = "baixando o tradutor (uma vez só)…";
        icone.remove();
      }
      var motor = await obterTradutor(idioma, function (porcento) {
        if (porcento) rotulo.textContent = "baixando o tradutor (uma vez só): " + porcento + "%";
      });
      var traducao = (await motor.translate(texto)).trim();
      status.remove();
      var resultado = document.createElement("p");
      resultado.className = "text-[13px] leading-5";
      resultado.textContent = traducao;
      div.appendChild(resultado);
      linhaDeAcoes(div, traducao);
      gravarCache(texto, traducao);
    } catch (erro) {
      status.textContent = "Não consegui traduzir offline: " + (erro && erro.message ? erro.message : "erro");
    }
  }

  // exposto para o leitor de EPUB (barra de seleção do lumina.js)
  window.luminaTraduzir = function (texto, rect) {
    fecharCartao();
    esconderBotao();
    traduzir((texto || "").trim(), rect);
  };

  // PDF: a camada de texto por cima do canvas permite selecionar; mostramos o
  // botão perto da seleção.
  if (areaPdf) {
    function selecaoNoPdf() {
      var selecao = window.getSelection();
      if (!selecao || selecao.isCollapsed || !selecao.rangeCount) return null;
      var texto = (selecao.toString() || "").trim();
      if (!texto) return null;
      var range = selecao.getRangeAt(0);
      var ancora = range.commonAncestorContainer;
      if (!areaPdf.contains(ancora.nodeType === 1 ? ancora : ancora.parentElement)) return null;
      return { texto: texto.slice(0, 1200), rect: range.getBoundingClientRect() };
    }

    document.addEventListener("pointerup", function () {
      setTimeout(function () {
        var achado = selecaoNoPdf();
        if (achado) mostrarBotao(achado.texto, achado.rect);
        else esconderBotao();
      }, 10);
    });
    document.addEventListener("selectionchange", function () {
      var selecao = window.getSelection();
      if (!selecao || selecao.isCollapsed) esconderBotao();
    });
    document.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") {
        esconderBotao();
        fecharCartao();
      }
    });
    document.addEventListener(
      "click",
      function (ev) {
        if (cartao && !cartao.contains(ev.target) && ev.target !== botao) fecharCartao();
      },
      true
    );
    window.addEventListener("scroll", esconderBotao, { passive: true });
  }
})();
