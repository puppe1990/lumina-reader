/* Leitor de PDF do Lumina (pdf.js): uma página por tela na horizontal (swipe
   com snap, toque nas bordas e setas), tema por canvas, progresso por página,
   marcador e busca no texto do PDF. */
(function () {
  "use strict";

  var root = document.querySelector("[data-pdf-root]");
  if (!root || typeof pdfjsLib === "undefined") return;

  var livroId = root.getAttribute("data-book");
  var paginaInicial = parseInt(root.getAttribute("data-page") || "1", 10) || 1;
  var CHAVE_CANVAS = "lumina-canvas";

  function token() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
  }

  function toast(mensagem) {
    var host = document.getElementById("amarra-toast-host");
    if (!host) return;
    var el = document.createElement("div");
    el.className =
      "fixed left-1/2 top-4 z-[60] -translate-x-1/2 rounded-full bg-inverse-surface px-4 py-2 text-[13px] text-inverse-on-surface shadow-lg";
    el.textContent = mensagem;
    host.appendChild(el);
    setTimeout(function () {
      el.remove();
    }, 2400);
  }

  // tema (só o canvas: PDF não tem "tamanho de fonte")
  function aplicarTema(nome) {
    document.documentElement.setAttribute("data-canvas", nome);
    try {
      localStorage.setItem(CHAVE_CANVAS, nome);
    } catch (e) {}
    var opcoes = document.querySelectorAll("[data-canvas-opcao]");
    for (var i = 0; i < opcoes.length; i++) {
      var ativo = opcoes[i].getAttribute("data-canvas-opcao") === nome;
      opcoes[i].classList.toggle("ring-2", ativo);
      opcoes[i].classList.toggle("ring-primary", ativo);
    }
  }
  var temaSalvo = null;
  try {
    temaSalvo = localStorage.getItem(CHAVE_CANVAS);
  } catch (e) {}
  aplicarTema(temaSalvo || document.documentElement.getAttribute("data-canvas") || "sepia");
  var botoesTema = document.querySelectorAll("[data-canvas-opcao]");
  for (var t = 0; t < botoesTema.length; t++) {
    (function (botao) {
      botao.addEventListener("click", function () {
        aplicarTema(botao.getAttribute("data-canvas-opcao"));
      });
    })(botoesTema[t]);
  }

  var viewport = document.getElementById("pdf-paginas");
  var trilho = document.getElementById("pdf-trilho");
  var carregando = document.getElementById("pdf-carregando");
  var rotuloPagina = document.getElementById("pdf-pagina");
  var posicao = document.getElementById("pdf-posicao");
  var percentual = document.getElementById("pdf-percentual");
  var barra = document.getElementById("pdf-barra");

  var paginas = [];
  var pdfDoc = null;
  var paginaAtual = 1;
  var textosPorPagina = {};

  pdfjsLib.GlobalWorkerOptions.workerSrc = "/static/vendor/pdfjs/pdf.worker.min.js";

  pdfjsLib
    .getDocument({
      url: "/livros/" + livroId + "/arquivo",
      standardFontDataUrl: "/static/vendor/pdfjs/standard_fonts/",
    })
    .promise.then(function (pdf) {
      pdfDoc = pdf;
      carregando.classList.add("hidden");
      for (var n = 1; n <= pdf.numPages; n++) {
        var slide = document.createElement("div");
        slide.className = "pdf-pagina flex h-full w-full shrink-0 items-center justify-center";
        slide.setAttribute("data-pagina", String(n));
        var caixa = document.createElement("div");
        caixa.className = "relative";
        var canvas = document.createElement("canvas");
        canvas.className = "pdf-folha block h-full w-full rounded bg-white shadow-md";
        caixa.appendChild(canvas);
        slide.appendChild(caixa);
        trilho.appendChild(slide);
        paginas.push({ numero: n, div: slide, caixa: caixa, canvas: canvas, renderizada: false, renderizando: false });
      }
      atualizarRotulos();
      irParaPagina(paginaInicial, false);
      manterJanela(paginaAtual);
    })
    .catch(function (erro) {
      carregando.innerHTML = "";
      var aviso = document.createElement("p");
      aviso.className = "px-6 py-10 text-center";
      aviso.textContent = "Não consegui abrir este PDF: " + (erro && erro.message ? erro.message : "erro desconhecido");
      carregando.appendChild(aviso);
    });

  // Spans invisíveis por cima do canvas: permitem selecionar o texto (traduzir
  // a seleção). O pdf.js 3 posiciona tudo pelo CSS var --scale-factor.
  function montarCamadaTexto(pagina, pg) {
    var anterior = pagina.caixa.querySelector(".textLayer");
    if (anterior) anterior.remove();
    var camada = document.createElement("div");
    camada.className = "textLayer";
    camada.style.setProperty("--scale-factor", String(pagina.escalaCss || 1));
    pagina.caixa.appendChild(camada);
    try {
      if (typeof pg.streamTextContent === "function") {
        return pdfjsLib
          .renderTextLayer({
            textContentSource: pg.streamTextContent({ includeMarkedContent: false }),
            container: camada,
            viewport: pg.getViewport({ scale: pagina.escalaCss || 1 }),
            textDivs: [],
          })
          .promise.catch(function () {});
      }
      return pg.getTextContent().then(function (conteudo) {
        return pdfjsLib.renderTextLayer({
          textContentSource: conteudo,
          container: camada,
          viewport: pg.getViewport({ scale: pagina.escalaCss || 1 }),
          textDivs: [],
        }).promise;
      }).catch(function () {});
    } catch (e) {
      return Promise.resolve();
    }
  }

  // A página é encaixada na tela (fit page): o texto fica legível sem zoom e a
  // virada mostra a página inteira, como num e-reader.
  function renderizarPagina(numero) {
    var pagina = paginas[numero - 1];
    if (!pagina || pagina.renderizada || pagina.renderizando || !pdfDoc) return;
    pagina.renderizando = true;
    pdfDoc
      .getPage(numero)
      .then(function (pg) {
        var base = pg.getViewport({ scale: 1 });
        var dpr = Math.min(window.devicePixelRatio || 1, 2);
        var larguraTela = Math.max(240, viewport.clientWidth - 28);
        var alturaTela = Math.max(240, viewport.clientHeight - 28);
        var escala = Math.min(larguraTela / base.width, alturaTela / base.height);
        var tela = pg.getViewport({ scale: escala * dpr });
        pagina.escalaCss = escala;
        pagina.canvas.width = Math.floor(tela.width);
        pagina.canvas.height = Math.floor(tela.height);
        // A caixa tem o tamanho CSS exato da página; canvas e camada de texto
        // ocupam 100% dela.
        pagina.caixa.style.width = Math.floor(tela.width / dpr) + "px";
        pagina.caixa.style.height = Math.floor(tela.height / dpr) + "px";
        return pg.render({ canvasContext: pagina.canvas.getContext("2d"), viewport: tela }).promise.then(function () {
          return montarCamadaTexto(pagina, pg);
        });
      })
      .then(function () {
        pagina.renderizada = true;
      })
      .catch(function () {
        pagina.renderizando = false;
      });
  }

  // Paginação por transform no trilho: sem depender de scroll nativo (o
  // scroll-snap do Chromium cancelava o salto e o smooth no meio do caminho).
  function irParaPagina(numero, suave) {
    numero = Math.max(1, Math.min(paginas.length || 1, numero));
    var mudou = numero !== paginaAtual;
    paginaAtual = numero;
    trilho.style.transition = suave === false ? "none" : "transform 260ms ease-out";
    trilho.style.transform = "translateX(" + -(numero - 1) * 100 + "%)";
    atualizarRotulos();
    manterJanela(numero);
    if (mudou) agendarSalvamento();
  }

  function proximaPagina() {
    irParaPagina(paginaAtual + 1, true);
  }
  function paginaAnterior() {
    irParaPagina(paginaAtual - 1, true);
  }

  // Janela de renderização: só as páginas perto da atual mantêm pixels.
  var JANELA_RENDER = 2;
  var JANELA_DESCARTE = 4;
  function manterJanela(atual) {
    for (var i = 0; i < paginas.length; i++) {
      var pagina = paginas[i];
      var distancia = Math.abs(pagina.numero - atual);
      if (distancia <= JANELA_RENDER && !pagina.renderizada && !pagina.renderizando) {
        renderizarPagina(pagina.numero);
      } else if (distancia > JANELA_DESCARTE && pagina.renderizada) {
        pagina.canvas.width = 0;
        pagina.canvas.height = 0;
        var camadaAntiga = pagina.caixa.querySelector(".textLayer");
        if (camadaAntiga) camadaAntiga.remove();
        pagina.renderizada = false;
        pagina.renderizando = false;
      }
    }
  }

  function atualizarRotulos() {
    var total = pdfDoc ? pdfDoc.numPages : 0;
    var percent = total ? Math.round((paginaAtual / total) * 100) : 0;
    if (rotuloPagina) rotuloPagina.textContent = "Página " + paginaAtual;
    if (posicao) posicao.textContent = total ? "Página " + paginaAtual + " de " + total : "Página " + paginaAtual;
    if (percentual) percentual.textContent = percent + "% concluído";
    if (barra) barra.style.width = percent + "%";
  }

  var salvarPendente = null;
  function salvarProgresso() {
    if (!pdfDoc) return;
    fetch("/livros/" + livroId + "/progresso", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": token() },
      body: JSON.stringify({ cap: paginaAtual - 1, ratio: 0, total: pdfDoc.numPages }),
    }).catch(function () {});
  }
  function agendarSalvamento() {
    clearTimeout(salvarPendente);
    salvarPendente = setTimeout(salvarProgresso, 1200);
  }

  // Swipe: arrasto horizontal no viewport vira página (pointer events cobrem
  // toque e mouse). O clique nas bordas continua valendo como atalho.
  var toqueInicio = null;
  var swipeConsumido = false;
  viewport.addEventListener("pointerdown", function (ev) {
    toqueInicio = { x: ev.clientX, y: ev.clientY, id: ev.pointerId };
  });
  viewport.addEventListener("pointerup", function (ev) {
    if (!toqueInicio) return;
    var dx = ev.clientX - toqueInicio.x;
    var dy = ev.clientY - toqueInicio.y;
    var mesmoPonteiro = ev.pointerId === toqueInicio.id;
    toqueInicio = null;
    if (!mesmoPonteiro) return;
    if (Math.abs(dx) < 48 || Math.abs(dx) < Math.abs(dy)) return; // clique/tap ou rolagem vertical
    swipeConsumido = true; // o clique sintético depois do arrasto não pode virar página de novo
    if (dx < 0) proximaPagina();
    else paginaAnterior();
  });
  viewport.addEventListener("pointercancel", function () {
    toqueInicio = null;
  });

  viewport.addEventListener("click", function (ev) {
    if (swipeConsumido) {
      swipeConsumido = false;
      return;
    }
    if (ev.target.closest("button") || ev.target.closest("a")) return;
    var rect = viewport.getBoundingClientRect();
    var x = ev.clientX - rect.left;
    if (x < rect.width * 0.28) paginaAnterior();
    else if (x > rect.width * 0.72) proximaPagina();
  });

  window.addEventListener("keydown", function (ev) {
    if (ev.key === "ArrowRight" || ev.key === "PageDown") proximaPagina();
    if (ev.key === "ArrowLeft" || ev.key === "PageUp") paginaAnterior();
  });

  var botoesVirada = document.querySelectorAll("[data-virada]");
  for (var b = 0; b < botoesVirada.length; b++) {
    (function (botao) {
      botao.addEventListener("click", function () {
        if (botao.getAttribute("data-virada") === "proxima") proximaPagina();
        else paginaAnterior();
      });
    })(botoesVirada[b]);
  }

  // Resize muda o encaixe da página: descarta os pixels e re-renderiza a janela.
  var timerResize = null;
  window.addEventListener("resize", function () {
    clearTimeout(timerResize);
    timerResize = setTimeout(function () {
      for (var i = 0; i < paginas.length; i++) {
        paginas[i].canvas.width = 0;
        paginas[i].canvas.height = 0;
        paginas[i].renderizada = false;
        paginas[i].renderizando = false;
      }
      manterJanela(paginaAtual);
      irParaPagina(paginaAtual, false);
    }, 250);
  });

  window.addEventListener("pagehide", function () {
    if (!pdfDoc) return;
    try {
      if (navigator.sendBeacon) {
        navigator.sendBeacon(
          "/livros/" + livroId + "/progresso",
          new Blob([JSON.stringify({ cap: paginaAtual - 1, ratio: 0, total: pdfDoc.numPages })], { type: "application/json" })
        );
      }
    } catch (e) {}
  });

  // marcador: mesma rota do leitor de EPUB
  var botaoMarcador = document.querySelector("[data-marcador]");
  if (botaoMarcador) {
    var iconeMarcador = document.getElementById("pdf-marcador-icone");
    botaoMarcador.addEventListener("click", function () {
      var ativo = botaoMarcador.getAttribute("aria-pressed") !== "true";
      botaoMarcador.setAttribute("aria-pressed", ativo ? "true" : "false");
      botaoMarcador.classList.toggle("text-primary", ativo);
      botaoMarcador.classList.toggle("text-on-surface-variant", !ativo);
      if (iconeMarcador) iconeMarcador.textContent = ativo ? "bookmark" : "bookmark_border";
      fetch("/livros/" + livroId + "/marcador", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": token() },
        body: JSON.stringify({ cap: paginaAtual - 1, ativo: ativo }),
      })
        .then(function () {
          toast(ativo ? "Marcador salvo nesta página" : "Marcador removido");
        })
        .catch(function () {
          toast("Não consegui salvar o marcador");
        });
    });
  }

  // Texto da página (para a busca e para o chat com o livro), cacheado.
  function textoDaPagina(numero) {
    if (textosPorPagina[numero] !== undefined) return Promise.resolve(textosPorPagina[numero]);
    if (!pdfDoc) return Promise.resolve("");
    return pdfDoc
      .getPage(numero)
      .then(function (pg) {
        return pg.getTextContent();
      })
      .then(function (conteudo) {
        var texto = conteudo.items
          .map(function (item) {
            return item.str || "";
          })
          .join(" ");
        texto = texto.replace(/\s+/g, " ").trim();
        textosPorPagina[numero] = texto;
        return texto;
      })
      .catch(function () {
        textosPorPagina[numero] = "";
        return "";
      });
  }

  function rotuloPosicao() {
    return "página " + paginaAtual + (pdfDoc ? " de " + pdfDoc.numPages : "");
  }

  // O "conversar com o livro" usa estes dois: a posição viva e o trecho atual.
  window.luminaPdf = {
    posicao: rotuloPosicao,
    textoDaPaginaAtual: function () {
      return textoDaPagina(paginaAtual);
    },
    contexto: function () {
      return textoDaPagina(paginaAtual).then(function (texto) {
        return { posicao: rotuloPosicao(), texto: texto || "" };
      });
    },
  };

  // busca no texto do PDF (getTextContent por página, cacheado)
  var campoBusca = document.getElementById("busca-termo");
  if (campoBusca) {
    var statusBusca = document.getElementById("busca-status");
    var listaBusca = document.getElementById("busca-hits");
    var timerBusca = null;

    function recorte(texto, pos, tamanho) {
      var inicio = Math.max(0, pos - 60);
      var fim = Math.min(texto.length, pos + tamanho + 60);
      return (inicio > 0 ? "…" : "") + texto.slice(inicio, fim).trim() + (fim < texto.length ? "…" : "");
    }

    function executarBusca() {
      var termo = campoBusca.value.trim();
      if (!pdfDoc || termo.length < 2) {
        listaBusca.innerHTML = "";
        statusBusca.textContent = termo ? "Digite pelo menos 2 letras." : "";
        return;
      }
      statusBusca.textContent = "Buscando no texto…";
      listaBusca.innerHTML = "";
      var alvo = termo.toLowerCase();
      var achados = 0;
      var pagina = 1;

      function varrer() {
        if (pagina > pdfDoc.numPages || achados >= 40) {
          statusBusca.textContent = achados ? achados + " trecho(s) encontrados" : "Nada encontrado (PDFs digitalizados não têm texto).";
          return;
        }
        if (pagina % 20 === 0) statusBusca.textContent = "Buscando… página " + pagina + " de " + pdfDoc.numPages;
        textoDaPagina(pagina).then(function (texto) {
          var baixo = texto.toLowerCase();
          var inicio = 0;
          var naPagina = 0;
          while (naPagina < 3) {
            var pos = baixo.indexOf(alvo, inicio);
            if (pos < 0) break;
            adicionarResultado(pagina, recorte(texto, pos, alvo.length));
            achados++;
            naPagina++;
            inicio = pos + alvo.length;
            if (achados >= 40) break;
          }
          pagina++;
          varrer();
        });
      }

      function adicionarResultado(numeroPagina, trecho) {
        var item = document.createElement("li");
        var botao = document.createElement("button");
        botao.type = "button";
        botao.className = "block w-full rounded-lg px-3 py-2 text-left transition-colors hover:bg-surface-container-low";
        var titulo = document.createElement("span");
        titulo.className = "block text-[10px] font-semibold uppercase tracking-wider text-primary";
        titulo.textContent = "Página " + numeroPagina;
        var corpo = document.createElement("span");
        corpo.className = "mt-0.5 block text-[13px] leading-5 text-on-surface";
        corpo.textContent = trecho;
        botao.appendChild(titulo);
        botao.appendChild(corpo);
        botao.addEventListener("click", function () {
          irParaPagina(numeroPagina, true);
          var dialogo = document.getElementById("dlg-busca");
          if (dialogo) dialogo.close();
        });
        item.appendChild(botao);
        listaBusca.appendChild(item);
      }

      varrer();
    }

    campoBusca.addEventListener("input", function () {
      clearTimeout(timerBusca);
      timerBusca = setTimeout(executarBusca, 350);
    });
    campoBusca.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") {
        ev.preventDefault();
        clearTimeout(timerBusca);
        executarBusca();
      }
    });
  }
})();
