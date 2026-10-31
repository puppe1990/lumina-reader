/* Leitor de PDF do Lumina (pdf.js): páginas em scroll contínuo, tema por
   canvas, progresso por página, marcador e busca no texto do PDF. */
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

  var container = document.getElementById("pdf-paginas");
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
      var observador = new IntersectionObserver(
        function (entradas) {
          entradas.forEach(function (entrada) {
            if (!entrada.isIntersecting) return;
            renderizarPagina(parseInt(entrada.target.getAttribute("data-pagina"), 10));
          });
        },
        { rootMargin: "900px 0px" }
      );
      for (var n = 1; n <= pdf.numPages; n++) {
        var div = document.createElement("div");
        div.className = "pdf-pagina relative w-full overflow-hidden rounded bg-white shadow-md";
        div.setAttribute("data-pagina", String(n));
        div.style.aspectRatio = "1 / 1.414";
        var canvas = document.createElement("canvas");
        canvas.className = "block h-auto w-full";
        div.appendChild(canvas);
        container.appendChild(div);
        paginas.push({ numero: n, div: div, canvas: canvas, renderizada: false, renderizando: false });
        observador.observe(div);
      }
      atualizarRotulos();
      if (paginaInicial > 1) {
        setTimeout(function () {
          irParaPagina(paginaInicial, false);
        }, 60);
      }
    })
    .catch(function (erro) {
      carregando.innerHTML = "";
      var aviso = document.createElement("p");
      aviso.textContent = "Não consegui abrir este PDF: " + (erro && erro.message ? erro.message : "erro desconhecido");
      carregando.appendChild(aviso);
    });

  function renderizarPagina(numero) {
    var pagina = paginas[numero - 1];
    if (!pagina || pagina.renderizada || pagina.renderizando || !pdfDoc) return;
    pagina.renderizando = true;
    pdfDoc
      .getPage(numero)
      .then(function (pg) {
        var base = pg.getViewport({ scale: 1 });
        var largura = Math.min(container.clientWidth, 900);
        var dpr = Math.min(window.devicePixelRatio || 1, 2);
        var viewport = pg.getViewport({ scale: (largura / base.width) * dpr });
        pagina.canvas.width = Math.floor(viewport.width);
        pagina.canvas.height = Math.floor(viewport.height);
        pagina.div.style.aspectRatio = base.width + " / " + base.height;
        return pg.render({ canvasContext: pagina.canvas.getContext("2d"), viewport: viewport }).promise;
      })
      .then(function () {
        pagina.renderizada = true;
      })
      .catch(function () {
        pagina.renderizando = false;
      });
  }

  function irParaPagina(numero, suave) {
    var pagina = paginas[numero - 1];
    if (!pagina) return;
    pagina.div.scrollIntoView({ block: "start", behavior: suave === false ? "auto" : "smooth" });
  }

  function atualizarRotulos() {
    var total = pdfDoc ? pdfDoc.numPages : 0;
    var percent = total ? Math.round((paginaAtual / total) * 100) : 0;
    if (rotuloPagina) rotuloPagina.textContent = "Página " + paginaAtual;
    if (posicao) posicao.textContent = total ? "Página " + paginaAtual + " de " + total : "Página " + paginaAtual;
    if (percentual) percentual.textContent = percent + "% concluído";
    if (barra) barra.style.width = percent + "%";
  }

  function paginaMaisVisivel() {
    var melhor = paginaAtual;
    var area = 0;
    var alturaTela = window.innerHeight;
    for (var i = 0; i < paginas.length; i++) {
      var rect = paginas[i].div.getBoundingClientRect();
      var visivel = Math.min(rect.bottom, alturaTela) - Math.max(rect.top, 0);
      if (visivel > area) {
        area = visivel;
        melhor = paginas[i].numero;
      }
    }
    return melhor;
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

  window.addEventListener(
    "scroll",
    function () {
      if (!pdfDoc) return;
      var atual = paginaMaisVisivel();
      if (atual === paginaAtual) return;
      paginaAtual = atual;
      atualizarRotulos();
      agendarSalvamento();
    },
    { passive: true }
  );
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

  // busca no texto do PDF (getTextContent por página, cacheado)
  var campoBusca = document.getElementById("busca-termo");
  if (campoBusca) {
    var statusBusca = document.getElementById("busca-status");
    var listaBusca = document.getElementById("busca-hits");
    var timerBusca = null;

    function textoDaPagina(numero) {
      if (textosPorPagina[numero] !== undefined) return Promise.resolve(textosPorPagina[numero]);
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
          statusBusca.textContent = achados
            ? achados + " trecho(s) encontrados"
            : "Nada encontrado (PDFs digitalizados não têm texto).";
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
