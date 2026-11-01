/* Tradução da seleção: selecionou uma palavra/trecho no leitor (EPUB ou PDF),
   aparece um botão "Traduzir" e o resultado num cartão flutuante. Usa o mesmo
   provedor do chat (trabalho em background + polling). */
(function () {
  "use strict";

  var areaPdf = document.getElementById("pdf-paginas");
  var botao = null;
  var cartao = null;
  var origem = null;

  function token() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
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

  function cartaoBase() {
    fecharCartao();
    var div = document.createElement("div");
    div.className =
      "fixed z-[70] w-[min(92vw,22rem)] rounded-2xl border border-outline-variant/50 bg-surface p-3 text-on-surface shadow-xl";
    document.body.appendChild(div);
    cartao = div;
    return div;
  }

  function rodape(container) {
    var linha = document.createElement("div");
    linha.className = "mt-2 flex items-center justify-between text-[11px] text-secondary";
    var copiar = document.createElement("button");
    copiar.type = "button";
    copiar.className = "rounded-full px-2 py-1 hover:bg-surface-container";
    copiar.textContent = "Copiar";
    var fechar = document.createElement("button");
    fechar.type = "button";
    fechar.className = "rounded-full px-2 py-1 hover:bg-surface-container";
    fechar.textContent = "Fechar";
    linha.appendChild(copiar);
    linha.appendChild(fechar);
    container.appendChild(linha);
    return { copiar: copiar, fechar: fechar };
  }

  async function traduzir(texto, rect) {
    if (!texto) return;
    var div = cartaoBase();
    var fonte = document.createElement("p");
    fonte.className = "mb-2 max-h-16 overflow-hidden text-[11px] italic leading-4 text-secondary";
    fonte.textContent = texto.length > 220 ? texto.slice(0, 220) + "…" : texto;
    var corpo = document.createElement("p");
    corpo.className = "flex items-center gap-2 text-[13px] leading-5";
    corpo.innerHTML =
      '<span class="material-symbols-outlined animate-spin text-[16px] text-primary">progress_activity</span>traduzindo…';
    div.appendChild(fonte);
    div.appendChild(corpo);
    if (rect) posicionar(div, rect);

    try {
      var resposta = await fetch("/traduzir", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": token() },
        body: JSON.stringify({ texto: texto }),
      });
      var dados = await resposta.json().catch(function () {
        return {};
      });
      if (dados.traducao) {
        // resposta veio do cache do servidor
        corpo.textContent = dados.traducao;
        var acoesCache = rodape(div);
        acoesCache.copiar.addEventListener("click", function () {
          try {
            navigator.clipboard.writeText(dados.traducao);
            acoesCache.copiar.textContent = "Copiado";
          } catch (e) {}
        });
        acoesCache.fechar.addEventListener("click", fecharCartao);
        return;
      }
      if (!resposta.ok || !dados.id) {
        corpo.textContent = dados.erro || "Não consegui traduzir agora.";
        return;
      }
      var inicio = Date.now();
      var lentoAvisado = false;
      while (Date.now() - inicio < 180000) {
        await new Promise(function (ok) {
          setTimeout(ok, 1200);
        });
        if (!lentoAvisado && Date.now() - inicio > 20000) {
          lentoAvisado = true;
          corpo.innerHTML =
            '<span class="material-symbols-outlined animate-spin text-[16px] text-primary">progress_activity</span>o provedor está lento, aguarde…';
        }
        var consulta = await fetch("/traduzir/" + encodeURIComponent(dados.id));
        var estado = await consulta.json().catch(function () {
          return {};
        });
        if (estado.estado && estado.estado !== "rodando") {
          if (estado.estado === "pronto" && estado.traducao) {
            corpo.textContent = estado.traducao;
            var acoes = rodape(div);
            acoes.copiar.addEventListener("click", function () {
              try {
                navigator.clipboard.writeText(estado.traducao);
                acoes.copiar.textContent = "Copiado";
              } catch (e) {}
            });
            acoes.fechar.addEventListener("click", fecharCartao);
          } else {
            corpo.textContent = estado.erro || "Não consegui traduzir agora.";
          }
          return;
        }
      }
      corpo.textContent = "A tradução demorou demais; tente de novo.";
    } catch (erro) {
      corpo.textContent = "Não consegui falar com o servidor.";
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
    document.addEventListener("click", function (ev) {
      if (cartao && !cartao.contains(ev.target) && ev.target !== botao) fecharCartao();
    }, true);
    window.addEventListener("scroll", esconderBotao, { passive: true });
  }
})();
