/* Lumina — interações do leitor: tema/tipografia, posição de leitura, pulso de
   tempo, marcador, seleção → destaque e realce das notas salvas. O leitor usa
   páginas com navegação completa (data-amarra-skip), então este arquivo roda no
   load; ainda assim re-init em amarra:morphed para qualquer página Drive. */
(function () {
  "use strict";

  var TAMANHOS = [16, 17, 18, 20, 22];
  var CHAVE_CANVAS = "lumina-canvas";
  var CHAVE_TAMANHO = "lumina-tamanho";

  function lerStorage(chave) {
    try {
      return localStorage.getItem(chave);
    } catch (e) {
      return null;
    }
  }
  function gravarStorage(chave, valor) {
    try {
      localStorage.setItem(chave, valor);
    } catch (e) {}
  }

  function token() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
  }

  function enviar(url, corpo) {
    return fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": token() },
      body: JSON.stringify(corpo),
    }).then(function (resposta) {
      if (!resposta.ok) throw new Error("status " + resposta.status);
      return resposta.json();
    });
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

  // ── índice de texto para casar/destacar trechos ──────────────────────────
  function normalizar(texto) {
    return (texto || "").replace(/\s+/g, " ").trim();
  }

  function indiceDeTexto(raiz) {
    var mapa = [];
    var texto = "";
    var walker = document.createTreeWalker(raiz, NodeFilter.SHOW_TEXT, null);
    var ultimoEspaco = true;
    var node;
    while ((node = walker.nextNode())) {
      if (node.parentElement && node.parentElement.closest("mark.nota")) continue;
      var valor = node.nodeValue || "";
      for (var i = 0; i < valor.length; i++) {
        var c = valor.charAt(i);
        if (/\s/.test(c)) {
          if (ultimoEspaco) continue;
          texto += " ";
          mapa.push([node, i]);
          ultimoEspaco = true;
        } else {
          texto += c;
          mapa.push([node, i]);
          ultimoEspaco = false;
        }
      }
    }
    return { texto: texto, mapa: mapa };
  }

  function destacarTrecho(raiz, excerpt, cor, idNota) {
    var agulha = normalizar(excerpt);
    if (agulha.length < 3) return null;
    var indice = indiceDeTexto(raiz);
    var pos = indice.texto.indexOf(agulha);
    if (pos < 0) return null;
    var inicio = indice.mapa[pos];
    var fim = indice.mapa[pos + agulha.length - 1];

    var mark = document.createElement("mark");
    mark.className = "nota nota-" + cor;
    if (idNota) mark.setAttribute("data-nota", idNota);

    if (inicio[0] === fim[0]) {
      var range = document.createRange();
      range.setStart(inicio[0], inicio[1]);
      range.setEnd(fim[0], fim[1] + 1);
      try {
        range.surroundContents(mark);
        return mark;
      } catch (e) {
        return null;
      }
    }
    // Trecho cruza elementos inline (<em>, <a>): realça só o primeiro nó,
    // sem quebrar a árvore.
    var parcial = document.createRange();
    parcial.setStart(inicio[0], inicio[1]);
    parcial.setEnd(inicio[0], (inicio[0].nodeValue || "").length);
    try {
      parcial.surroundContents(mark);
      return mark;
    } catch (e) {
      return null;
    }
  }

  // ── leitor ───────────────────────────────────────────────────────────────
  function iniciarLeitor(root) {
    var canvas = root.querySelector(".leitor-canvas");
    var artigo = root.querySelector(".leitor-artigo");
    if (!canvas || !artigo) return;

    var livroId = root.getAttribute("data-book");
    var cap = parseInt(root.getAttribute("data-cap") || "0", 10);
    var baseUrl = "/livros/" + livroId;

    // tema + tamanho (o head aplica antes do paint; aqui só sincroniza os controles)
    var temaAtual = document.documentElement.getAttribute("data-canvas") || lerStorage(CHAVE_CANVAS) || "sepia";
    var tamanhoAtual = parseInt(lerStorage(CHAVE_TAMANHO) || "2", 10);
    if (isNaN(tamanhoAtual)) tamanhoAtual = 2;
    aplicarTema(temaAtual, root);
    aplicarTamanho(tamanhoAtual, root);

    function aplicarTema(nome, escopo) {
      document.documentElement.setAttribute("data-canvas", nome);
      gravarStorage(CHAVE_CANVAS, nome);
      var marcados = (escopo || document).querySelectorAll("[data-canvas-opcao]");
      for (var i = 0; i < marcados.length; i++) {
        var ativo = marcados[i].getAttribute("data-canvas-opcao") === nome;
        marcados[i].classList.toggle("ring-2", ativo);
        marcados[i].classList.toggle("ring-primary", ativo);
      }
    }
    function aplicarTamanho(indice, escopo) {
      tamanhoAtual = Math.max(0, Math.min(TAMANHOS.length - 1, indice));
      document.documentElement.style.setProperty("--reader-font-size", TAMANHOS[tamanhoAtual] + "px");
      gravarStorage(CHAVE_TAMANHO, String(tamanhoAtual));
      var pontos = (escopo || document).querySelectorAll("[data-tamanho-pontos] span");
      for (var i = 0; i < pontos.length; i++) {
        pontos[i].classList.toggle("bg-primary", i <= tamanhoAtual);
        pontos[i].classList.toggle("bg-surface-container-highest", i > tamanhoAtual);
      }
    }

    var controlesTema = root.querySelectorAll("[data-canvas-opcao]");
    for (var i = 0; i < controlesTema.length; i++) {
      (function (botao) {
        botao.addEventListener("click", function () {
          aplicarTema(botao.getAttribute("data-canvas-opcao"), root);
        });
      })(controlesTema[i]);
    }
    var botaoMenos = root.querySelector('[data-tamanho="menos"]');
    var botaoMais = root.querySelector('[data-tamanho="mais"]');
    if (botaoMenos) botaoMenos.addEventListener("click", function () { aplicarTamanho(tamanhoAtual - 1, root); });
    if (botaoMais) botaoMais.addEventListener("click", function () { aplicarTamanho(tamanhoAtual + 1, root); });

    // posição de leitura -----------------------------------------------------
    function ratioVisivel() {
      var doc = document.documentElement;
      var altura = Math.max(1, doc.scrollHeight - window.innerHeight);
      return Math.max(0, Math.min(1, window.scrollY / altura));
    }
    var ultimoSalvo = -1;
    var salvarPendente = null;
    function salvarProgresso() {
      var ratio = ratioVisivel();
      enviar(baseUrl + "/progresso", { cap: cap, ratio: ratio })
        .then(function (resposta) {
          var alvo = document.getElementById("percentual");
          if (alvo && typeof resposta.percent === "number") {
            alvo.textContent = Math.round(resposta.percent) + "% concluído";
          }
        })
        .catch(function () {});
    }
    function agendarSalvamento() {
      if (salvarPendente) clearTimeout(salvarPendente);
      salvarPendente = setTimeout(function () {
        if (Math.abs(ratioVisivel() - ultimoSalvo) > 0.01) {
          ultimoSalvo = ratioVisivel();
          salvarProgresso();
        }
      }, 1200);
    }
    window.addEventListener("scroll", agendarSalvamento, { passive: true });
    window.addEventListener("pagehide", function () {
      var ratio = ratioVisivel();
      var corpo = JSON.stringify({ cap: cap, ratio: ratio });
      try {
        if (navigator.sendBeacon) {
          navigator.sendBeacon(baseUrl + "/progresso", new Blob([corpo], { type: "application/json" }));
        }
      } catch (e) {}
    });

    // restaurar posição ------------------------------------------------------
    var restaurar = parseFloat(root.getAttribute("data-restore") || "0");
    var notaPedida = new URLSearchParams(location.search).get("nota");
    if (restaurar > 0.02) {
      requestAnimationFrame(function () {
        var altura = document.documentElement.scrollHeight - window.innerHeight;
        window.scrollTo(0, Math.max(0, altura * restaurar));
      });
    }

    // pulso de tempo ----------------------------------------------------------
    setInterval(function () {
      if (document.visibilityState === "visible" && !document.hidden) {
        enviar(baseUrl + "/pulso", { segundos: 30 }).catch(function () {});
      }
    }, 30000);

    // marcador ----------------------------------------------------------------
    var botaoMarcador = root.querySelector("[data-marcador]");
    if (botaoMarcador) {
      botaoMarcador.addEventListener("click", function () {
        var ativo = botaoMarcador.getAttribute("aria-pressed") !== "true";
        botaoMarcador.setAttribute("aria-pressed", ativo ? "true" : "false");
        botaoMarcador.classList.toggle("text-primary", ativo);
        botaoMarcador.classList.toggle("text-on-surface-variant", !ativo);
        var icone = botaoMarcador.querySelector(".material-symbols-outlined");
        if (icone) {
          icone.classList.toggle("filled", ativo);
          icone.style.fontVariationSettings = ativo ? "'FILL' 1" : "'FILL' 0";
        }
        enviar(baseUrl + "/marcador", { cap: cap, ativo: ativo })
          .then(function () {
            toast(ativo ? "Marcador salvo neste capítulo" : "Marcador removido");
          })
          .catch(function () {
            toast("Não consegui salvar o marcador");
          });
      });
    }

    // realce das notas salvas -------------------------------------------------
    var notas = [];
    var dados = document.getElementById("leitor-notas");
    if (dados) {
      try {
        notas = JSON.parse(dados.textContent || "[]") || [];
      } catch (e) {
        notas = [];
      }
    }
    if (!Array.isArray(notas)) notas = [];

    // capitular: só no primeiro parágrafo "de verdade" do capítulo
    var primeiroP = artigo.querySelector(".leitor-texto p");
    if (primeiroP && (primeiroP.textContent || "").trim().length > 140) {
      primeiroP.classList.add("capitular");
    }

    var marcados = {};
    for (var n = 0; n < notas.length; n++) {
      var el = destacarTrecho(artigo, notas[n].Excerpt, notas[n].Color, notas[n].ID);
      if (el) marcados[String(notas[n].ID)] = el;
    }
    if (notaPedida && marcados[notaPedida]) {
      setTimeout(function () {
        marcados[notaPedida].scrollIntoView({ block: "center" });
      }, 120);
    }

    // seleção → barra de destaque --------------------------------------------
    var barra = document.getElementById("barra-selecao");
    var selecaoAtual = null;
    function esconderBarra() {
      if (barra) barra.classList.add("hidden");
    }
    function posicionarBarra(range) {
      if (!barra) return;
      var rect = range.getBoundingClientRect();
      var largura = barra.offsetWidth || 320;
      var esquerda = Math.min(Math.max(rect.left + rect.width / 2, largura / 2 + 8), window.innerWidth - largura / 2 - 8);
      var topo = rect.top - 52;
      if (topo < 68) topo = rect.bottom + 12;
      barra.style.left = esquerda + "px";
      barra.style.top = topo + "px";
      barra.classList.remove("hidden");
      barra.classList.add("flex");
    }
    if (barra) {
      barra.addEventListener("mousedown", function (ev) {
        ev.preventDefault(); // preserva a seleção enquanto o clique acontece
      });
      barra.addEventListener("click", function (ev) {
        var alvo = ev.target.closest("button");
        if (!alvo || !selecaoAtual) return;
        var trecho = normalizar(selecaoAtual.toString());
        if (!trecho) return;
        var cor = alvo.getAttribute("data-destaque");
        var acao = alvo.getAttribute("data-acao");

        if (acao === "copiar") {
          try {
            navigator.clipboard.writeText(selecaoAtual.toString());
            toast("Trecho copiado");
          } catch (e) {
            toast("Não consegui copiar");
          }
          esconderBarra();
          return;
        }
        if (acao === "nota") {
          var anotacao = window.prompt("Sua nota sobre o trecho:");
          if (anotacao === null) return;
          enviar(baseUrl + "/notas", { cap: cap, cor: "amarelo", trecho: trecho, nota: anotacao })
            .then(function (resposta) {
              destacarTrecho(artigo, trecho, "amarelo", resposta.id);
              toast("Nota salva no caderno");
            })
            .catch(function () {});
          esconderBarra();
          return;
        }
        if (cor) {
          enviar(baseUrl + "/notas", { cap: cap, cor: cor, trecho: trecho })
            .then(function (resposta) {
              destacarTrecho(artigo, trecho, cor, resposta.id);
              toast("Destaque salvo no caderno");
            })
            .catch(function () {
              toast("Não consegui salvar o destaque");
            });
          esconderBarra();
        }
      });
    }
    document.addEventListener("selectionchange", function () {
      setTimeout(function () {
        var sel = window.getSelection();
        if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
          esconderBarra();
          return;
        }
        var range = sel.getRangeAt(0);
        if (!artigo.contains(range.commonAncestorContainer)) {
          esconderBarra();
          return;
        }
        selecaoAtual = sel;
        posicionarBarra(range);
      }, 10);
    });
  }

  function iniciar() {
    var raiz = document.querySelector("[data-leitor]");
    if (raiz) iniciarLeitor(raiz);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", iniciar);
  } else {
    iniciar();
  }
  document.addEventListener("amarra:morphed", iniciar);
})();
