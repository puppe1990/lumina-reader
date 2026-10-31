/* amarra-cais v0.25.1 */
(() => {
  var __defProp = Object.defineProperty;
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };

  // pkg/amarra/js/vendor/idiomorph.js
  var Idiomorph = (function() {
    "use strict";
    const noOp = () => {
    };
    const defaults = {
      morphStyle: "outerHTML",
      callbacks: {
        beforeNodeAdded: noOp,
        afterNodeAdded: noOp,
        beforeNodeMorphed: noOp,
        afterNodeMorphed: noOp,
        beforeNodeRemoved: noOp,
        afterNodeRemoved: noOp,
        beforeAttributeUpdated: noOp
      },
      head: {
        style: "merge",
        shouldPreserve: (elt) => elt.getAttribute("im-preserve") === "true",
        shouldReAppend: (elt) => elt.getAttribute("im-re-append") === "true",
        shouldRemove: noOp,
        afterHeadMorphed: noOp
      },
      restoreFocus: true
    };
    function morph2(oldNode, newContent, config = {}) {
      oldNode = normalizeElement(oldNode);
      const newNode = normalizeParent(newContent);
      const ctx = createMorphContext(oldNode, newNode, config);
      const morphedNodes = saveAndRestoreFocus(ctx, () => {
        return withHeadBlocking(
          ctx,
          oldNode,
          newNode,
          /** @param {MorphContext} ctx */
          (ctx2) => {
            if (ctx2.morphStyle === "innerHTML") {
              morphChildren(ctx2, oldNode, newNode);
              return Array.from(oldNode.childNodes);
            } else {
              return morphOuterHTML(ctx2, oldNode, newNode);
            }
          }
        );
      });
      ctx.pantry.remove();
      return morphedNodes;
    }
    function morphOuterHTML(ctx, oldNode, newNode) {
      const oldParent = normalizeParent(oldNode);
      morphChildren(
        ctx,
        oldParent,
        newNode,
        // these two optional params are the secret sauce
        oldNode,
        // start point for iteration
        oldNode.nextSibling
        // end point for iteration
      );
      return Array.from(oldParent.childNodes);
    }
    function saveAndRestoreFocus(ctx, fn) {
      if (!ctx.config.restoreFocus) return fn();
      let activeElement = (
        /** @type {HTMLInputElement|HTMLTextAreaElement|null} */
        document.activeElement
      );
      if (!(activeElement instanceof HTMLInputElement || activeElement instanceof HTMLTextAreaElement)) {
        return fn();
      }
      const { id: activeElementId, selectionStart, selectionEnd } = activeElement;
      const results = fn();
      if (activeElementId && activeElementId !== document.activeElement?.getAttribute("id")) {
        activeElement = ctx.target.querySelector(`[id="${activeElementId}"]`);
        activeElement?.focus();
      }
      if (activeElement && !activeElement.selectionEnd && selectionEnd) {
        activeElement.setSelectionRange(selectionStart, selectionEnd);
      }
      return results;
    }
    const morphChildren = /* @__PURE__ */ (function() {
      function morphChildren2(ctx, oldParent, newParent, insertionPoint = null, endPoint = null) {
        if (oldParent instanceof HTMLTemplateElement && newParent instanceof HTMLTemplateElement) {
          oldParent = oldParent.content;
          newParent = newParent.content;
        }
        insertionPoint ||= oldParent.firstChild;
        for (const newChild of newParent.childNodes) {
          if (insertionPoint && insertionPoint != endPoint) {
            const bestMatch = findBestMatch(
              ctx,
              newChild,
              insertionPoint,
              endPoint
            );
            if (bestMatch) {
              if (bestMatch !== insertionPoint) {
                removeNodesBetween(ctx, insertionPoint, bestMatch);
              }
              morphNode(bestMatch, newChild, ctx);
              insertionPoint = bestMatch.nextSibling;
              continue;
            }
          }
          if (newChild instanceof Element) {
            const newChildId = (
              /** @type {String} */
              newChild.getAttribute("id")
            );
            if (ctx.persistentIds.has(newChildId)) {
              const movedChild = moveBeforeById(
                oldParent,
                newChildId,
                insertionPoint,
                ctx
              );
              morphNode(movedChild, newChild, ctx);
              insertionPoint = movedChild.nextSibling;
              continue;
            }
          }
          const insertedNode = createNode(
            oldParent,
            newChild,
            insertionPoint,
            ctx
          );
          if (insertedNode) {
            insertionPoint = insertedNode.nextSibling;
          }
        }
        while (insertionPoint && insertionPoint != endPoint) {
          const tempNode = insertionPoint;
          insertionPoint = insertionPoint.nextSibling;
          removeNode(ctx, tempNode);
        }
      }
      function createNode(oldParent, newChild, insertionPoint, ctx) {
        if (ctx.callbacks.beforeNodeAdded(newChild) === false) return null;
        if (ctx.idMap.has(newChild)) {
          const newEmptyChild = document.createElement(
            /** @type {Element} */
            newChild.tagName
          );
          oldParent.insertBefore(newEmptyChild, insertionPoint);
          morphNode(newEmptyChild, newChild, ctx);
          ctx.callbacks.afterNodeAdded(newEmptyChild);
          return newEmptyChild;
        } else {
          const newClonedChild = document.importNode(newChild, true);
          oldParent.insertBefore(newClonedChild, insertionPoint);
          ctx.callbacks.afterNodeAdded(newClonedChild);
          return newClonedChild;
        }
      }
      const findBestMatch = /* @__PURE__ */ (function() {
        function findBestMatch2(ctx, node, startPoint, endPoint) {
          let softMatch = null;
          let nextSibling = node.nextSibling;
          let siblingSoftMatchCount = 0;
          let cursor = startPoint;
          while (cursor && cursor != endPoint) {
            if (isSoftMatch(cursor, node)) {
              if (isIdSetMatch(ctx, cursor, node)) {
                return cursor;
              }
              if (softMatch === null) {
                if (!ctx.idMap.has(cursor)) {
                  softMatch = cursor;
                }
              }
            }
            if (softMatch === null && nextSibling && isSoftMatch(cursor, nextSibling)) {
              siblingSoftMatchCount++;
              nextSibling = nextSibling.nextSibling;
              if (siblingSoftMatchCount >= 2) {
                softMatch = void 0;
              }
            }
            if (ctx.activeElementAndParents.includes(cursor)) break;
            cursor = cursor.nextSibling;
          }
          return softMatch || null;
        }
        function isIdSetMatch(ctx, oldNode, newNode) {
          let oldSet = ctx.idMap.get(oldNode);
          let newSet = ctx.idMap.get(newNode);
          if (!newSet || !oldSet) return false;
          for (const id of oldSet) {
            if (newSet.has(id)) {
              return true;
            }
          }
          return false;
        }
        function isSoftMatch(oldNode, newNode) {
          const oldElt = (
            /** @type {Element} */
            oldNode
          );
          const newElt = (
            /** @type {Element} */
            newNode
          );
          return oldElt.nodeType === newElt.nodeType && oldElt.tagName === newElt.tagName && // If oldElt has an `id` with possible state and it doesn't match newElt.id then avoid morphing.
          // We'll still match an anonymous node with an IDed newElt, though, because if it got this far,
          // its not persistent, and new nodes can't have any hidden state.
          // We can't use .id because of form input shadowing, and we can't count on .getAttribute's presence because it could be a document-fragment
          (!oldElt.getAttribute?.("id") || oldElt.getAttribute?.("id") === newElt.getAttribute?.("id"));
        }
        return findBestMatch2;
      })();
      function removeNode(ctx, node) {
        if (ctx.idMap.has(node)) {
          moveBefore(ctx.pantry, node, null);
        } else {
          if (ctx.callbacks.beforeNodeRemoved(node) === false) return;
          node.parentNode?.removeChild(node);
          ctx.callbacks.afterNodeRemoved(node);
        }
      }
      function removeNodesBetween(ctx, startInclusive, endExclusive) {
        let cursor = startInclusive;
        while (cursor && cursor !== endExclusive) {
          let tempNode = (
            /** @type {Node} */
            cursor
          );
          cursor = cursor.nextSibling;
          removeNode(ctx, tempNode);
        }
        return cursor;
      }
      function moveBeforeById(parentNode, id, after, ctx) {
        const target = (
          /** @type {Element} - will always be found */
          // ctx.target.id unsafe because of form input shadowing
          // ctx.target could be a document fragment which doesn't have `getAttribute`
          ctx.target.getAttribute?.("id") === id && ctx.target || ctx.target.querySelector(`[id="${id}"]`) || ctx.pantry.querySelector(`[id="${id}"]`)
        );
        removeElementFromAncestorsIdMaps(target, ctx);
        moveBefore(parentNode, target, after);
        return target;
      }
      function removeElementFromAncestorsIdMaps(element, ctx) {
        const id = (
          /** @type {String} */
          element.getAttribute("id")
        );
        while (element = element.parentNode) {
          let idSet = ctx.idMap.get(element);
          if (idSet) {
            idSet.delete(id);
            if (!idSet.size) {
              ctx.idMap.delete(element);
            }
          }
        }
      }
      function moveBefore(parentNode, element, after) {
        if (parentNode.moveBefore) {
          try {
            parentNode.moveBefore(element, after);
          } catch (e) {
            parentNode.insertBefore(element, after);
          }
        } else {
          parentNode.insertBefore(element, after);
        }
      }
      return morphChildren2;
    })();
    const morphNode = /* @__PURE__ */ (function() {
      function morphNode2(oldNode, newContent, ctx) {
        if (ctx.ignoreActive && oldNode === document.activeElement) {
          return null;
        }
        if (ctx.callbacks.beforeNodeMorphed(oldNode, newContent) === false) {
          return oldNode;
        }
        if (oldNode instanceof HTMLHeadElement && ctx.head.ignore) {
        } else if (oldNode instanceof HTMLHeadElement && ctx.head.style !== "morph") {
          handleHeadElement(
            oldNode,
            /** @type {HTMLHeadElement} */
            newContent,
            ctx
          );
        } else {
          morphAttributes(oldNode, newContent, ctx);
          if (!ignoreValueOfActiveElement(oldNode, ctx)) {
            morphChildren(ctx, oldNode, newContent);
          }
        }
        ctx.callbacks.afterNodeMorphed(oldNode, newContent);
        return oldNode;
      }
      function morphAttributes(oldNode, newNode, ctx) {
        let type = newNode.nodeType;
        if (type === 1) {
          const oldElt = (
            /** @type {Element} */
            oldNode
          );
          const newElt = (
            /** @type {Element} */
            newNode
          );
          const oldAttributes = oldElt.attributes;
          const newAttributes = newElt.attributes;
          for (const newAttribute of newAttributes) {
            if (ignoreAttribute(newAttribute.name, oldElt, "update", ctx)) {
              continue;
            }
            if (oldElt.getAttribute(newAttribute.name) !== newAttribute.value) {
              oldElt.setAttribute(newAttribute.name, newAttribute.value);
            }
          }
          for (let i = oldAttributes.length - 1; 0 <= i; i--) {
            const oldAttribute = oldAttributes[i];
            if (!oldAttribute) continue;
            if (!newElt.hasAttribute(oldAttribute.name)) {
              if (ignoreAttribute(oldAttribute.name, oldElt, "remove", ctx)) {
                continue;
              }
              oldElt.removeAttribute(oldAttribute.name);
            }
          }
          if (!ignoreValueOfActiveElement(oldElt, ctx)) {
            syncInputValue(oldElt, newElt, ctx);
          }
        }
        if (type === 8 || type === 3) {
          if (oldNode.nodeValue !== newNode.nodeValue) {
            oldNode.nodeValue = newNode.nodeValue;
          }
        }
      }
      function syncInputValue(oldElement, newElement, ctx) {
        if (oldElement instanceof HTMLInputElement && newElement instanceof HTMLInputElement && newElement.type !== "file") {
          let newValue = newElement.value;
          let oldValue = oldElement.value;
          syncBooleanAttribute(oldElement, newElement, "checked", ctx);
          syncBooleanAttribute(oldElement, newElement, "disabled", ctx);
          if (!newElement.hasAttribute("value")) {
            if (!ignoreAttribute("value", oldElement, "remove", ctx)) {
              oldElement.value = "";
              oldElement.removeAttribute("value");
            }
          } else if (oldValue !== newValue) {
            if (!ignoreAttribute("value", oldElement, "update", ctx)) {
              oldElement.setAttribute("value", newValue);
              oldElement.value = newValue;
            }
          }
        } else if (oldElement instanceof HTMLOptionElement && newElement instanceof HTMLOptionElement) {
          syncBooleanAttribute(oldElement, newElement, "selected", ctx);
        } else if (oldElement instanceof HTMLTextAreaElement && newElement instanceof HTMLTextAreaElement) {
          let newValue = newElement.value;
          let oldValue = oldElement.value;
          if (ignoreAttribute("value", oldElement, "update", ctx)) {
            return;
          }
          if (newValue !== oldValue) {
            oldElement.value = newValue;
          }
          if (oldElement.firstChild && oldElement.firstChild.nodeValue !== newValue) {
            oldElement.firstChild.nodeValue = newValue;
          }
        }
      }
      function syncBooleanAttribute(oldElement, newElement, attributeName, ctx) {
        const newLiveValue = newElement[attributeName], oldLiveValue = oldElement[attributeName];
        if (newLiveValue !== oldLiveValue) {
          const ignoreUpdate = ignoreAttribute(
            attributeName,
            oldElement,
            "update",
            ctx
          );
          if (!ignoreUpdate) {
            oldElement[attributeName] = newElement[attributeName];
          }
          if (newLiveValue) {
            if (!ignoreUpdate) {
              oldElement.setAttribute(attributeName, "");
            }
          } else {
            if (!ignoreAttribute(attributeName, oldElement, "remove", ctx)) {
              oldElement.removeAttribute(attributeName);
            }
          }
        }
      }
      function ignoreAttribute(attr, element, updateType, ctx) {
        if (attr === "value" && ctx.ignoreActiveValue && element === document.activeElement) {
          return true;
        }
        return ctx.callbacks.beforeAttributeUpdated(attr, element, updateType) === false;
      }
      function ignoreValueOfActiveElement(possibleActiveElement, ctx) {
        return !!ctx.ignoreActiveValue && possibleActiveElement === document.activeElement && possibleActiveElement !== document.body;
      }
      return morphNode2;
    })();
    function withHeadBlocking(ctx, oldNode, newNode, callback) {
      if (ctx.head.block) {
        const oldHead = oldNode.querySelector("head");
        const newHead = newNode.querySelector("head");
        if (oldHead && newHead) {
          const promises = handleHeadElement(oldHead, newHead, ctx);
          return Promise.all(promises).then(() => {
            const newCtx = Object.assign(ctx, {
              head: {
                block: false,
                ignore: true
              }
            });
            return callback(newCtx);
          });
        }
      }
      return callback(ctx);
    }
    function handleHeadElement(oldHead, newHead, ctx) {
      let added = [];
      let removed = [];
      let preserved = [];
      let nodesToAppend = [];
      let srcToNewHeadNodes = /* @__PURE__ */ new Map();
      for (const newHeadChild of newHead.children) {
        srcToNewHeadNodes.set(newHeadChild.outerHTML, newHeadChild);
      }
      for (const currentHeadElt of oldHead.children) {
        let inNewContent = srcToNewHeadNodes.has(currentHeadElt.outerHTML);
        let isReAppended = ctx.head.shouldReAppend(currentHeadElt);
        let isPreserved = ctx.head.shouldPreserve(currentHeadElt);
        if (inNewContent || isPreserved) {
          if (isReAppended) {
            removed.push(currentHeadElt);
          } else {
            srcToNewHeadNodes.delete(currentHeadElt.outerHTML);
            preserved.push(currentHeadElt);
          }
        } else {
          if (ctx.head.style === "append") {
            if (isReAppended) {
              removed.push(currentHeadElt);
              nodesToAppend.push(currentHeadElt);
            }
          } else {
            if (ctx.head.shouldRemove(currentHeadElt) !== false) {
              removed.push(currentHeadElt);
            }
          }
        }
      }
      nodesToAppend.push(...srcToNewHeadNodes.values());
      let promises = [];
      for (const newNode of nodesToAppend) {
        let newElt = (
          /** @type {ChildNode} */
          document.createRange().createContextualFragment(newNode.outerHTML).firstChild
        );
        if (ctx.callbacks.beforeNodeAdded(newElt) !== false) {
          if ("href" in newElt && newElt.href || "src" in newElt && newElt.src) {
            let resolve;
            let promise = new Promise(function(_resolve) {
              resolve = _resolve;
            });
            newElt.addEventListener("load", function() {
              resolve();
            });
            promises.push(promise);
          }
          oldHead.appendChild(newElt);
          ctx.callbacks.afterNodeAdded(newElt);
          added.push(newElt);
        }
      }
      for (const removedElement of removed) {
        if (ctx.callbacks.beforeNodeRemoved(removedElement) !== false) {
          oldHead.removeChild(removedElement);
          ctx.callbacks.afterNodeRemoved(removedElement);
        }
      }
      ctx.head.afterHeadMorphed(oldHead, {
        added,
        kept: preserved,
        removed
      });
      return promises;
    }
    const createMorphContext = /* @__PURE__ */ (function() {
      function createMorphContext2(oldNode, newContent, config) {
        const { persistentIds, idMap } = createIdMaps(oldNode, newContent);
        const mergedConfig = mergeDefaults(config);
        const morphStyle = mergedConfig.morphStyle || "outerHTML";
        if (!["innerHTML", "outerHTML"].includes(morphStyle)) {
          throw `Do not understand how to morph style ${morphStyle}`;
        }
        return {
          target: oldNode,
          newContent,
          config: mergedConfig,
          morphStyle,
          ignoreActive: mergedConfig.ignoreActive,
          ignoreActiveValue: mergedConfig.ignoreActiveValue,
          restoreFocus: mergedConfig.restoreFocus,
          idMap,
          persistentIds,
          pantry: createPantry(),
          activeElementAndParents: createActiveElementAndParents(oldNode),
          callbacks: mergedConfig.callbacks,
          head: mergedConfig.head
        };
      }
      function mergeDefaults(config) {
        let finalConfig = Object.assign({}, defaults);
        Object.assign(finalConfig, config);
        finalConfig.callbacks = Object.assign(
          {},
          defaults.callbacks,
          config.callbacks
        );
        finalConfig.head = Object.assign({}, defaults.head, config.head);
        return finalConfig;
      }
      function createPantry() {
        const pantry = document.createElement("div");
        pantry.hidden = true;
        document.body.insertAdjacentElement("afterend", pantry);
        return pantry;
      }
      function createActiveElementAndParents(oldNode) {
        let activeElementAndParents = [];
        let elt = document.activeElement;
        if (elt?.tagName !== "BODY" && oldNode.contains(elt)) {
          while (elt) {
            activeElementAndParents.push(elt);
            if (elt === oldNode) break;
            elt = elt.parentElement;
          }
        }
        return activeElementAndParents;
      }
      function findIdElements(root) {
        let elements = Array.from(root.querySelectorAll("[id]"));
        if (root.getAttribute?.("id")) {
          elements.push(root);
        }
        return elements;
      }
      function populateIdMapWithTree(idMap, persistentIds, root, elements) {
        for (const elt of elements) {
          const id = (
            /** @type {String} */
            elt.getAttribute("id")
          );
          if (persistentIds.has(id)) {
            let current = elt;
            while (current) {
              let idSet = idMap.get(current);
              if (idSet == null) {
                idSet = /* @__PURE__ */ new Set();
                idMap.set(current, idSet);
              }
              idSet.add(id);
              if (current === root) break;
              current = current.parentElement;
            }
          }
        }
      }
      function createIdMaps(oldContent, newContent) {
        const oldIdElements = findIdElements(oldContent);
        const newIdElements = findIdElements(newContent);
        const persistentIds = createPersistentIds(oldIdElements, newIdElements);
        let idMap = /* @__PURE__ */ new Map();
        populateIdMapWithTree(idMap, persistentIds, oldContent, oldIdElements);
        const newRoot = newContent.__idiomorphRoot || newContent;
        populateIdMapWithTree(idMap, persistentIds, newRoot, newIdElements);
        return { persistentIds, idMap };
      }
      function createPersistentIds(oldIdElements, newIdElements) {
        let duplicateIds = /* @__PURE__ */ new Set();
        let oldIdTagNameMap = /* @__PURE__ */ new Map();
        for (const { id, tagName } of oldIdElements) {
          if (oldIdTagNameMap.has(id)) {
            duplicateIds.add(id);
          } else {
            oldIdTagNameMap.set(id, tagName);
          }
        }
        let persistentIds = /* @__PURE__ */ new Set();
        for (const { id, tagName } of newIdElements) {
          if (persistentIds.has(id)) {
            duplicateIds.add(id);
          } else if (oldIdTagNameMap.get(id) === tagName) {
            persistentIds.add(id);
          }
        }
        for (const id of duplicateIds) {
          persistentIds.delete(id);
        }
        return persistentIds;
      }
      return createMorphContext2;
    })();
    const { normalizeElement, normalizeParent } = /* @__PURE__ */ (function() {
      const generatedByIdiomorph = /* @__PURE__ */ new WeakSet();
      function normalizeElement2(content) {
        if (content instanceof Document) {
          return content.documentElement;
        } else {
          return content;
        }
      }
      function normalizeParent2(newContent) {
        if (newContent == null) {
          return document.createElement("div");
        } else if (typeof newContent === "string") {
          return normalizeParent2(parseContent(newContent));
        } else if (generatedByIdiomorph.has(
          /** @type {Element} */
          newContent
        )) {
          return (
            /** @type {Element} */
            newContent
          );
        } else if (newContent instanceof Node) {
          if (newContent.parentNode) {
            return (
              /** @type {any} */
              new SlicedParentNode(newContent)
            );
          } else {
            const dummyParent = document.createElement("div");
            dummyParent.append(newContent);
            return dummyParent;
          }
        } else {
          const dummyParent = document.createElement("div");
          for (const elt of [...newContent]) {
            dummyParent.append(elt);
          }
          return dummyParent;
        }
      }
      class SlicedParentNode {
        /** @param {Node} node */
        constructor(node) {
          this.originalNode = node;
          this.realParentNode = /** @type {Element} */
          node.parentNode;
          this.previousSibling = node.previousSibling;
          this.nextSibling = node.nextSibling;
        }
        /** @returns {Node[]} */
        get childNodes() {
          const nodes = [];
          let cursor = this.previousSibling ? this.previousSibling.nextSibling : this.realParentNode.firstChild;
          while (cursor && cursor != this.nextSibling) {
            nodes.push(cursor);
            cursor = cursor.nextSibling;
          }
          return nodes;
        }
        /**
         * @param {string} selector
         * @returns {Element[]}
         */
        querySelectorAll(selector) {
          return this.childNodes.reduce(
            (results, node) => {
              if (node instanceof Element) {
                if (node.matches(selector)) results.push(node);
                const nodeList = node.querySelectorAll(selector);
                for (let i = 0; i < nodeList.length; i++) {
                  results.push(nodeList[i]);
                }
              }
              return results;
            },
            /** @type {Element[]} */
            []
          );
        }
        /**
         * @param {Node} node
         * @param {Node} referenceNode
         * @returns {Node}
         */
        insertBefore(node, referenceNode) {
          return this.realParentNode.insertBefore(node, referenceNode);
        }
        /**
         * @param {Node} node
         * @param {Node} referenceNode
         * @returns {Node}
         */
        moveBefore(node, referenceNode) {
          return this.realParentNode.moveBefore(node, referenceNode);
        }
        /**
         * for later use with populateIdMapWithTree to halt upwards iteration
         * @returns {Node}
         */
        get __idiomorphRoot() {
          return this.originalNode;
        }
      }
      function parseContent(newContent) {
        let parser = new DOMParser();
        let contentWithSvgsRemoved = newContent.replace(
          /<svg(\s[^>]*>|>)([\s\S]*?)<\/svg>/gim,
          ""
        );
        if (contentWithSvgsRemoved.match(/<\/html>/) || contentWithSvgsRemoved.match(/<\/head>/) || contentWithSvgsRemoved.match(/<\/body>/)) {
          let content = parser.parseFromString(newContent, "text/html");
          if (contentWithSvgsRemoved.match(/<\/html>/)) {
            generatedByIdiomorph.add(content);
            return content;
          } else {
            let htmlElement = content.firstChild;
            if (htmlElement) {
              generatedByIdiomorph.add(htmlElement);
            }
            return htmlElement;
          }
        } else {
          let responseDoc = parser.parseFromString(
            "<body><template>" + newContent + "</template></body>",
            "text/html"
          );
          let content = (
            /** @type {HTMLTemplateElement} */
            responseDoc.body.querySelector("template").content
          );
          generatedByIdiomorph.add(content);
          return content;
        }
      }
      return { normalizeElement: normalizeElement2, normalizeParent: normalizeParent2 };
    })();
    return {
      morph: morph2,
      defaults
    };
  })();

  // pkg/amarra/js/drive.mjs
  var drive_exports = {};
  __export(drive_exports, {
    applyDriveResponse: () => applyDriveResponse,
    driveHeaders: () => driveHeaders,
    extractMainHTML: () => extractMainHTML,
    extractMainTagName: () => extractMainTagName,
    shouldInterceptClick: () => shouldInterceptClick,
    shouldInterceptSubmit: () => shouldInterceptSubmit,
    start: () => start3,
    visit: () => visit
  });

  // pkg/amarra/js/svg_attrs.mjs
  var SVG_NS = "http://www.w3.org/2000/svg";
  var CAMEL_CASE_SVG_ATTRS = [
    "attributeName",
    "attributeType",
    "baseFrequency",
    "baseProfile",
    "calcMode",
    "clipPathUnits",
    "diffuseConstant",
    "edgeMode",
    "filterUnits",
    "glyphRef",
    "gradientTransform",
    "gradientUnits",
    "kernelMatrix",
    "kernelUnitLength",
    "keyPoints",
    "keySplines",
    "keyTimes",
    "lengthAdjust",
    "limitingConeAngle",
    "markerHeight",
    "markerUnits",
    "markerWidth",
    "maskContentUnits",
    "maskUnits",
    "numOctaves",
    "pathLength",
    "patternContentUnits",
    "patternTransform",
    "patternUnits",
    "pointsAtX",
    "pointsAtY",
    "pointsAtZ",
    "preserveAlpha",
    "preserveAspectRatio",
    "primitiveUnits",
    "refX",
    "refY",
    "repeatCount",
    "repeatDur",
    "requiredExtensions",
    "requiredFeatures",
    "specularConstant",
    "specularExponent",
    "spreadMethod",
    "startOffset",
    "stdDeviation",
    "stitchTiles",
    "surfaceScale",
    "systemLanguage",
    "tableValues",
    "targetX",
    "targetY",
    "textLength",
    "viewBox",
    "viewTarget",
    "xChannelSelector",
    "yChannelSelector",
    "zoomAndPan"
  ];
  var canonicalByLowerCase = new Map(
    CAMEL_CASE_SVG_ATTRS.map((name) => [name.toLowerCase(), name])
  );
  function restoreSvgAttributeCase(root) {
    if (!root || typeof root.querySelectorAll !== "function") return 0;
    let restored = 0;
    for (const element of [root, ...root.querySelectorAll("*")]) {
      if (!isSvgElement(element)) continue;
      for (const attribute of [...element.attributes ?? []]) {
        const canonical = canonicalByLowerCase.get(String(attribute.name).toLowerCase());
        if (!canonical || attribute.name === canonical) continue;
        element.setAttribute(canonical, attribute.value);
        element.removeAttribute(attribute.name);
        restored += 1;
      }
    }
    return restored;
  }
  function isSvgElement(element) {
    return !!element && element.namespaceURI === SVG_NS && typeof element.setAttribute === "function" && typeof element.removeAttribute === "function";
  }

  // pkg/amarra/js/morph.mjs
  function morph(el, html, morphFn, opts = {}) {
    if (!el) return;
    const parent = el.parentNode ?? null;
    const result = morphInto(el, html, morphFn, opts);
    restoreSvgAttributeCase(el.isConnected === false && parent ? parent : el);
    return result;
  }
  function morphInto(el, html, morphFn, opts) {
    if (typeof morphFn === "function") return morphFn(el, html);
    const style = opts.morphStyle || "innerHTML";
    const lib = globalThis.Idiomorph;
    if (lib && typeof lib.morph === "function") {
      return lib.morph(el, html, { morphStyle: style });
    }
    if (style === "outerHTML" && "outerHTML" in el) {
      el.outerHTML = html ?? "";
      return;
    }
    if ("innerHTML" in el) el.innerHTML = html ?? "";
  }

  // pkg/amarra/js/sequence.mjs
  var counters = /* @__PURE__ */ new WeakMap();
  var fallback = 0;
  function bumpSequence(key) {
    if (!key || typeof key !== "object") {
      fallback += 1;
      return fallback;
    }
    const next = (counters.get(key) ?? 0) + 1;
    counters.set(key, next);
    return next;
  }
  function isCurrentSequence(key, seq) {
    if (!key || typeof key !== "object") return seq === fallback;
    return seq === (counters.get(key) ?? 0);
  }

  // pkg/amarra/js/hook.mjs
  var hook_exports = {};
  __export(hook_exports, {
    afterMorph: () => afterMorph,
    applyFocus: () => applyFocus,
    applyOptimistic: () => applyOptimistic,
    csrfTokenFromMeta: () => csrfTokenFromMeta,
    dispatchLivePush: () => dispatchLivePush,
    register: () => register,
    reset: () => reset,
    rollbackOptimistic: () => rollbackOptimistic,
    scan: () => scan,
    showToast: () => showToast,
    start: () => start
  });

  // pkg/amarra/js/hook_registry.mjs
  var defs = /* @__PURE__ */ new Map();
  var mounted = /* @__PURE__ */ new Map();
  var auto = /* @__PURE__ */ new Map();
  function register(name, def) {
    if (!name || !def) return;
    defs.set(name, def);
  }
  function registerAuto(tag, name) {
    if (tag && name) auto.set(String(tag).toUpperCase(), name);
  }
  function reset() {
    defs.clear();
    mounted.clear();
  }
  function scan(root) {
    if (!root) return;
    const found = collect(root);
    const seen = new Set(found);
    for (const el of found) {
      const name = hookName(el);
      const def = defs.get(name);
      const cur = mounted.get(el);
      if (cur && cur.name === name) {
        def?.updated?.(el);
        continue;
      }
      if (cur) {
        cur.def.disconnect?.(el);
        mounted.delete(el);
      }
      if (!def) continue;
      mounted.set(el, { name, def });
      def.connect?.(el);
    }
    for (const [el, cur] of [...mounted]) {
      if (seen.has(el)) continue;
      const orphan = el.isConnected === false || typeof root.contains === "function" && root.contains(el);
      if (!orphan) continue;
      cur.def.disconnect?.(el);
      mounted.delete(el);
    }
  }
  function hookName(el) {
    const explicit = el.getAttribute?.("amarra-hook") || "";
    if (explicit) return explicit;
    return auto.get(el.tagName?.toUpperCase?.()) ?? "";
  }
  function dispatchLivePush(event, payload) {
    for (const [el, cur] of mounted) {
      cur.def.handleEvent?.(event, payload, el);
    }
  }
  function collect(root) {
    const out = [];
    const push = (el) => {
      if (el && !out.includes(el)) out.push(el);
    };
    if (root.hasAttribute?.("amarra-hook") || auto.has(root.tagName?.toUpperCase?.())) push(root);
    for (const sel of ["[amarra-hook]", ...auto.keys()]) {
      const list = root.querySelectorAll?.(sel);
      if (!list) continue;
      for (const el of list) push(el);
    }
    return out;
  }

  // pkg/amarra/js/hook_bulk.mjs
  var STATE = "_amarraBulkState";
  function makeBulk() {
    return {
      connect(el) {
        if (!el?.querySelector) return;
        const all = el.querySelector("[data-amarra-bulk-all]");
        const rows = [...el.querySelectorAll?.("[data-amarra-bulk-row]") ?? []];
        const bar = el.querySelector("[data-amarra-bulk-bar]");
        const count = bar?.querySelector?.("[data-amarra-bulk-count]");
        if (!all || rows.length === 0) return;
        const sync = () => {
          const selected = rows.filter((r) => r.checked).length;
          all.indeterminate = selected > 0 && selected < rows.length;
          all.checked = selected === rows.length;
          if (bar) {
            bar.hidden = selected === 0;
            bar.classList?.toggle("hidden", selected === 0);
            bar.classList?.toggle("flex", selected !== 0);
            if (count) count.textContent = String(selected);
          }
        };
        const onAll = () => {
          for (const row of rows) row.checked = all.checked;
          sync();
        };
        all.addEventListener?.("change", onAll);
        const rowUnbinds = [];
        for (const row of rows) {
          const fn = () => sync();
          row.addEventListener?.("change", fn);
          rowUnbinds.push([row, fn]);
        }
        el[STATE] = { all, onAll, rowUnbinds };
      },
      // #126: Idiomorph keeps the container and swaps children; rebind so the
      // hook tracks the new nodes instead of stale references.
      updated(el) {
        bulk.disconnect(el);
        bulk.connect(el);
      },
      disconnect(el) {
        const st = el?.[STATE];
        if (!st) return;
        st.all.removeEventListener?.("change", st.onAll);
        for (const [row, fn] of st.rowUnbinds) row.removeEventListener?.("change", fn);
        delete el[STATE];
      }
    };
  }
  var bulk = makeBulk();

  // pkg/amarra/js/hook_clipboard.mjs
  var CLICK = "_amarraClipboardClick";
  function makeClipboard(writeText) {
    return {
      connect(el) {
        if (!el || typeof el.addEventListener !== "function") return;
        const fn = (ev) => {
          ev?.preventDefault?.();
          const text = el.getAttribute?.("data-amarra-copy") ?? "";
          writeText?.(text);
        };
        el[CLICK] = fn;
        el.addEventListener("click", fn);
      },
      disconnect(el) {
        const fn = el?.[CLICK];
        if (!fn || typeof el.removeEventListener !== "function") return;
        el.removeEventListener("click", fn);
        delete el[CLICK];
      }
    };
  }
  var clipboard = makeClipboard((text) => {
    const write = globalThis.navigator?.clipboard?.writeText;
    if (typeof write === "function") return write.call(globalThis.navigator.clipboard, text);
  });

  // pkg/amarra/js/hook_dialog.mjs
  var OPEN = "_amarraDialogOpen";
  var CLOSE = "_amarraDialogClose";
  var ONCLOSE = "_amarraDialogOnClose";
  var STATE2 = "_amarraDialogState";
  function makeDialog() {
    return {
      connect(el) {
        if (!el?.querySelector) return;
        const dlg = el.querySelector("[data-amarra-dialog-target]");
        if (!dlg || typeof dlg.showModal !== "function") return;
        if (!dlg.getAttribute?.("aria-modal")) dlg.setAttribute?.("aria-modal", "true");
        const openers = [...el.querySelectorAll?.("[data-amarra-dialog-open]") ?? []];
        const closers = [...el.querySelectorAll?.("[data-amarra-dialog-close]") ?? []];
        let opener = null;
        for (const btn of openers) {
          const fn = (ev) => {
            ev?.preventDefault?.();
            opener = btn;
            dlg.showModal?.();
          };
          btn[OPEN] = fn;
          btn.addEventListener?.("click", fn);
        }
        for (const btn of closers) {
          const fn = (ev) => {
            ev?.preventDefault?.();
            dlg.close?.();
          };
          btn[CLOSE] = fn;
          btn.addEventListener?.("click", fn);
        }
        const onClose = () => opener?.focus?.();
        dlg[ONCLOSE] = onClose;
        dlg.addEventListener?.("close", onClose);
        el[STATE2] = { dlg, openers, closers };
      },
      // #126: Idiomorph keeps the container and swaps children; rebind so the
      // hook tracks the new nodes instead of stale references.
      updated(el) {
        dialog.disconnect(el);
        dialog.connect(el);
      },
      disconnect(el) {
        const st = el?.[STATE2];
        if (!st) return;
        for (const btn of st.openers ?? []) {
          const fn = btn?.[OPEN];
          if (!fn) continue;
          btn.removeEventListener?.("click", fn);
          delete btn[OPEN];
        }
        for (const btn of st.closers ?? []) {
          const fn = btn?.[CLOSE];
          if (!fn) continue;
          btn.removeEventListener?.("click", fn);
          delete btn[CLOSE];
        }
        const onClose = st.dlg?.[ONCLOSE];
        if (onClose) {
          st.dlg.removeEventListener?.("close", onClose);
          delete st.dlg[ONCLOSE];
        }
        delete el[STATE2];
      }
    };
  }
  var dialog = makeDialog();

  // pkg/amarra/js/hook_dropdown.mjs
  var BTN = "_amarraDropdownToggle";
  var MENU = "_amarraDropdownMenuClick";
  var STATE3 = "_amarraDropdownState";
  function makeDropdown() {
    return {
      connect(el) {
        if (!el?.querySelector) return;
        const btn = el.querySelector("[data-amarra-dropdown-button]");
        const menu = el.querySelector("[data-amarra-dropdown-menu]");
        if (!btn || !menu) return;
        const doc = el.ownerDocument ?? globalThis.document;
        const close2 = () => {
          menu.hidden = true;
          btn.setAttribute?.("aria-expanded", "false");
        };
        const toggle = (ev) => {
          ev?.preventDefault?.();
          const open2 = menu.hidden;
          menu.hidden = !open2;
          btn.setAttribute?.("aria-expanded", String(open2));
        };
        const onDocClick = (ev) => {
          if (el.contains?.(ev?.target)) return;
          close2();
        };
        const onKey2 = (ev) => {
          if (ev?.key === "Escape") close2();
        };
        btn[BTN] = toggle;
        btn.addEventListener?.("click", toggle);
        menu[MENU] = close2;
        menu.addEventListener?.("click", close2);
        doc?.addEventListener?.("click", onDocClick);
        doc?.addEventListener?.("keydown", onKey2);
        el[STATE3] = { btn, menu, doc, onDocClick, onKey: onKey2 };
      },
      // #126: Idiomorph keeps the container and swaps children; rebind so the
      // hook tracks the new nodes and document listeners do not accumulate.
      updated(el) {
        dropdown.disconnect(el);
        dropdown.connect(el);
      },
      disconnect(el) {
        const st = el?.[STATE3];
        if (!st) return;
        const toggle = st.btn?.[BTN];
        if (toggle) {
          st.btn.removeEventListener?.("click", toggle);
          delete st.btn[BTN];
        }
        const menuClose = st.menu?.[MENU];
        if (menuClose) {
          st.menu.removeEventListener?.("click", menuClose);
          delete st.menu[MENU];
        }
        if (st.onDocClick) st.doc?.removeEventListener?.("click", st.onDocClick);
        if (st.onKey) st.doc?.removeEventListener?.("keydown", st.onKey);
        delete el[STATE3];
      }
    };
  }
  var dropdown = makeDropdown();

  // pkg/amarra/js/hook_nav.mjs
  var POPSTATE = "_amarraNavPopstate";
  function makeNav(opts = {}) {
    const getLocation = opts.location ?? (() => globalThis.location);
    const getWindow = opts.window ?? globalThis.window;
    function sync(el) {
      const loc = getLocation();
      if (!el || !loc?.pathname) return;
      const on = classes(el, "data-amarra-nav-on", opts.onClasses);
      const off = classes(el, "data-amarra-nav-off", opts.offClasses);
      for (const link of el.querySelectorAll?.("a[href]") ?? []) {
        if (isActive(link, loc)) {
          off.forEach((c) => link.classList?.remove(c));
          on.forEach((c) => link.classList?.add(c));
          link.setAttribute?.("aria-current", "page");
        } else {
          on.forEach((c) => link.classList?.remove(c));
          if (off.length) off.forEach((c) => link.classList?.add(c));
          link.removeAttribute?.("aria-current");
        }
      }
    }
    return {
      connect(el) {
        if (!el) return;
        sync(el);
        const onPop = () => sync(el);
        el[POPSTATE] = onPop;
        getWindow?.addEventListener?.("popstate", onPop);
      },
      updated(el) {
        sync(el);
      },
      disconnect(el) {
        const fn = el?.[POPSTATE];
        if (!fn) return;
        getWindow?.removeEventListener?.("popstate", fn);
        delete el[POPSTATE];
      }
    };
  }
  function classes(el, attr, fallback2) {
    const raw = el.getAttribute?.(attr) || fallback2;
    if (!raw) return [];
    return raw.split(/\s+/).filter(Boolean);
  }
  function isActive(link, loc) {
    const href = link.getAttribute?.("href");
    if (!href || href.startsWith("#")) return false;
    try {
      return new URL(href, loc.href).pathname === loc.pathname;
    } catch {
      return false;
    }
  }
  var nav = makeNav();

  // pkg/amarra/js/hook_password.mjs
  var CLICK2 = "_amarraPasswordClick";
  function makePassword(findInput) {
    const resolve = findInput ?? defaultFind;
    return {
      connect(el) {
        if (!el || typeof el.addEventListener !== "function") return;
        const fn = (ev) => {
          ev?.preventDefault?.();
          const sel = el.getAttribute?.("data-amarra-password-for") ?? "";
          const input = resolve(sel, el);
          if (!input) return;
          const show2 = input.type === "password";
          input.type = show2 ? "text" : "password";
          el.setAttribute?.("aria-pressed", show2 ? "true" : "false");
          swapAriaLabel(el, show2);
          toggleIcons(el, show2);
        };
        el[CLICK2] = fn;
        el.addEventListener("click", fn);
      },
      disconnect(el) {
        const fn = el?.[CLICK2];
        if (!fn || typeof el.removeEventListener !== "function") return;
        el.removeEventListener("click", fn);
        delete el[CLICK2];
      }
    };
  }
  function defaultFind(sel, el) {
    if (sel) {
      const root = el?.ownerDocument ?? globalThis.document;
      try {
        const found = root?.querySelector?.(sel);
        if (found) return found;
      } catch {
        return null;
      }
    }
    const scope = el?.closest?.("form") ?? el?.parentElement ?? null;
    const inputs = scope?.querySelectorAll?.('input[type="password"]') ?? [];
    if (inputs.length === 0) return null;
    if (inputs.length === 1) return inputs[0];
    let best = inputs[0];
    for (const input of inputs) {
      if (el?.compareDocumentPosition?.(input) & 2) best = input;
    }
    return best;
  }
  function swapAriaLabel(el, show2) {
    const showLabel = el.getAttribute?.("data-amarra-label-show");
    const hideLabel = el.getAttribute?.("data-amarra-label-hide");
    if (!showLabel && !hideLabel) return;
    el.setAttribute?.("aria-label", show2 ? hideLabel || showLabel : showLabel || hideLabel);
  }
  function toggleIcons(el, show2) {
    const showIcon = el.querySelector?.('[data-amarra-password-icon="show"]') ?? el.querySelector?.('[data-cais-password-icon="show"]');
    const hideIcon = el.querySelector?.('[data-amarra-password-icon="hide"]') ?? el.querySelector?.('[data-cais-password-icon="hide"]');
    showIcon?.classList?.toggle?.("hidden", show2);
    hideIcon?.classList?.toggle?.("hidden", !show2);
  }
  var password = makePassword();

  // pkg/amarra/js/hook_reveal.mjs
  var SYNC = "_amarraRevealSync";
  function makeReveal(findTarget) {
    const resolve = findTarget ?? defaultFind2;
    return {
      connect(el) {
        if (!el || typeof el.addEventListener !== "function") return;
        const fn = () => {
          const match = el.getAttribute?.("data-amarra-reveal-show") ?? "";
          const sel = el.getAttribute?.("data-amarra-reveal-target") ?? "";
          const target = resolve(sel, el);
          if (!target) return;
          target.hidden = el.value !== match;
        };
        el[SYNC] = fn;
        el.addEventListener("change", fn);
        el.addEventListener("click", fn);
        fn();
      },
      disconnect(el) {
        const fn = el?.[SYNC];
        if (!fn || typeof el.removeEventListener !== "function") return;
        el.removeEventListener("change", fn);
        el.removeEventListener("click", fn);
        delete el[SYNC];
      }
    };
  }
  function defaultFind2(sel, el) {
    if (!sel) return null;
    const root = el?.ownerDocument ?? globalThis.document;
    try {
      return root?.querySelector?.(sel) ?? null;
    } catch {
      return null;
    }
  }
  var reveal = makeReveal();

  // pkg/amarra/js/hook_select.mjs
  var STATE4 = "_amarraSelectSearch";
  var SR_CLASS = "cais-select-search-native";
  var listSeq = 0;
  function makeSelectSearch(deps = {}) {
    const isCoarse = deps.isCoarse ?? defaultIsCoarse;
    return {
      connect(el) {
        mount(el, isCoarse, deps);
      },
      updated(el) {
        unmount(el);
        mount(el, isCoarse, deps);
      },
      disconnect(el) {
        unmount(el);
      }
    };
  }
  var select = makeSelectSearch();
  function defaultIsCoarse(el) {
    const win = el?.ownerDocument?.defaultView;
    const coarse = win?.matchMedia?.("(pointer: coarse)")?.matches;
    return !!coarse && (win?.navigator?.maxTouchPoints ?? 0) > 0;
  }
  function mount(selectEl, isCoarse, deps) {
    if (!isEnhanceable(selectEl, isCoarse)) return;
    const doc = selectEl.ownerDocument;
    const state = buildUI(selectEl, doc);
    state.Observer = deps?.MutationObserver ?? null;
    bindEvents(selectEl, state);
    selectEl.classList.add(SR_CLASS);
    selectEl[STATE4] = state;
    syncLabel(selectEl, state);
  }
  function isEnhanceable(selectEl, isCoarse) {
    if (!selectEl || selectEl.tagName !== "SELECT" || selectEl.multiple) return false;
    if (selectEl[STATE4]) return false;
    if (!selectEl.parentNode || !selectEl.ownerDocument) return false;
    if (selectEl.getAttribute?.("data-amarra-select-search") === "false") return false;
    return !isCoarse(selectEl);
  }
  function buildUI(selectEl, doc) {
    const wrapper = doc.createElement("div");
    wrapper.classList.add("cais-select-search");
    const trigger = doc.createElement("button");
    trigger.setAttribute("type", "button");
    trigger.setAttribute("aria-haspopup", "listbox");
    trigger.setAttribute("aria-expanded", "false");
    trigger.classList.add("cais-select-search-trigger");
    const label = doc.createElement("span");
    label.classList.add("cais-select-search-label");
    const chevron = doc.createElement("span");
    chevron.classList.add("cais-select-search-chevron");
    chevron.setAttribute("aria-hidden", "true");
    chevron.textContent = "\u25BE";
    trigger.appendChild(label);
    trigger.appendChild(chevron);
    const panel = doc.createElement("div");
    panel.classList.add("cais-select-search-panel");
    panel.hidden = true;
    const listId = `amarra-select-list-${++listSeq}`;
    const list = doc.createElement("ul");
    list.setAttribute("role", "listbox");
    list.setAttribute("id", listId);
    list.classList.add("cais-select-search-list");
    trigger.setAttribute("aria-controls", listId);
    const input = doc.createElement("input");
    input.setAttribute("type", "text");
    input.setAttribute("autocomplete", "off");
    input.setAttribute("aria-label", searchPlaceholder(selectEl, doc));
    input.setAttribute("placeholder", searchPlaceholder(selectEl, doc));
    input.classList.add("cais-select-search-input");
    const rows = buildRows(selectEl, doc, list);
    panel.appendChild(input);
    panel.appendChild(list);
    wrapper.appendChild(trigger);
    wrapper.appendChild(panel);
    selectEl.parentNode.insertBefore(wrapper, selectEl.nextSibling);
    return { doc, wrapper, trigger, panel, input, list, rows, label };
  }
  function buildRows(selectEl, doc, list) {
    return Array.from(selectEl.options ?? []).map((option, index) => {
      const row = doc.createElement("li");
      row.setAttribute("role", "option");
      row.setAttribute("data-value", option.value ?? "");
      row.setAttribute("data-index", String(index));
      row.classList.add("cais-select-search-option");
      if (option.disabled) row.setAttribute("aria-disabled", "true");
      row.textContent = option.textContent ?? "";
      list.appendChild(row);
      return row;
    });
  }
  function bindEvents(selectEl, state) {
    const { trigger, input, panel, wrapper } = state;
    const doc = state.doc;
    state.onTriggerClick = (ev) => {
      ev?.preventDefault?.();
      if (panel.hidden) open(selectEl, state);
      else close(state);
    };
    state.onTriggerKey = (ev) => {
      if (ev?.key === "ArrowDown" || ev?.key === "Enter" || ev?.key === " ") {
        ev?.preventDefault?.();
        open(selectEl, state);
      }
    };
    state.onInput = () => filter(selectEl, state);
    state.onInputKey = (ev) => inputKey(selectEl, state, ev);
    state.onChange = () => syncLabel(selectEl, state);
    state.onDocClick = (ev) => {
      if (panel.hidden) return;
      if (wrapper.contains?.(ev?.target)) return;
      close(state);
    };
    bindRows(selectEl, state);
    observeOptions(selectEl, state);
    trigger.addEventListener("click", state.onTriggerClick);
    trigger.addEventListener("keydown", state.onTriggerKey);
    input.addEventListener("input", state.onInput);
    input.addEventListener("keydown", state.onInputKey);
    selectEl.addEventListener("change", state.onChange);
    doc.addEventListener("click", state.onDocClick, true);
  }
  function bindRows(selectEl, state) {
    state.rowClicks = state.rows.map((row) => {
      const fn = (ev) => {
        ev?.preventDefault?.();
        choose(selectEl, state, row);
      };
      row.addEventListener("click", fn);
      return fn;
    });
  }
  function unbindRows(state) {
    state.rows.forEach((row, index) => row.removeEventListener?.("click", state.rowClicks?.[index]));
  }
  function rebuildRows(selectEl, state) {
    unbindRows(state);
    for (const row of state.rows) row.parentNode?.removeChild?.(row);
    state.rows = buildRows(selectEl, state.doc, state.list);
    bindRows(selectEl, state);
    syncLabel(selectEl, state);
  }
  function observeOptions(selectEl, state) {
    const Observer = state.Observer ?? selectEl.ownerDocument?.defaultView?.MutationObserver;
    if (!Observer) return;
    state.observer = new Observer(() => rebuildRows(selectEl, state));
    state.observer.observe?.(selectEl, { childList: true });
  }
  function open(selectEl, state) {
    state.panel.hidden = false;
    state.trigger.setAttribute("aria-expanded", "true");
    state.input.value = "";
    filter(selectEl, state);
    state.input.focus?.();
  }
  function close(state) {
    state.panel.hidden = true;
    state.trigger.setAttribute("aria-expanded", "false");
  }
  function choose(selectEl, state, row) {
    selectEl.value = row.getAttribute("data-value") ?? "";
    dispatchChange(selectEl);
    syncLabel(selectEl, state);
    close(state);
    state.trigger.focus?.();
  }
  function dispatchChange(selectEl) {
    if (typeof selectEl.dispatchEvent === "function" && typeof Event === "function") {
      selectEl.dispatchEvent(new Event("change", { bubbles: true }));
      return;
    }
    selectEl.fire?.("change", { type: "change" });
  }
  function syncLabel(selectEl, state) {
    const options = Array.from(selectEl.options ?? []);
    const selected = options.find((o) => (o.value ?? "") === (selectEl.value ?? "")) ?? options[0];
    state.label.textContent = selected?.textContent ?? "";
    state.rows.forEach((row, index) => {
      const on = (options[index]?.value ?? "") === (selectEl.value ?? "");
      row.setAttribute("aria-selected", on ? "true" : "false");
      row.classList.toggle("is-selected", on);
    });
  }
  function filter(selectEl, state) {
    const query = normalize(state.input.value);
    let firstVisible = null;
    state.rows.forEach((row) => {
      const hidden = query !== "" && !normalize(row.textContent).includes(query);
      row.classList.toggle("is-hidden", hidden);
      if (!hidden && firstVisible === null) firstVisible = row;
    });
    highlight(state, firstVisible);
  }
  function highlight(state, row) {
    state.rows.forEach((r) => r.classList.toggle("is-highlighted", r === row));
    state.highlighted = row ?? null;
    row?.scrollIntoView?.({ block: "nearest" });
  }
  function moveHighlight(state, delta) {
    const visible = state.rows.filter((r) => !r.classList.contains("is-hidden"));
    if (!visible.length) return;
    let current = visible.indexOf(state.highlighted);
    if (current < 0) current = delta > 0 ? -1 : visible.length;
    const next = Math.max(0, Math.min(visible.length - 1, current + delta));
    highlight(state, visible[next]);
  }
  function inputKey(selectEl, state, ev) {
    const key = ev?.key;
    if (key === "ArrowDown") {
      ev?.preventDefault?.();
      moveHighlight(state, 1);
    } else if (key === "ArrowUp") {
      ev?.preventDefault?.();
      moveHighlight(state, -1);
    } else if (key === "Enter") {
      ev?.preventDefault?.();
      if (state.highlighted) choose(selectEl, state, state.highlighted);
    } else if (key === "Escape") {
      ev?.preventDefault?.();
      close(state);
      state.trigger.focus?.();
    } else if (key === "Tab") {
      close(state);
    }
  }
  function unmount(selectEl) {
    const state = selectEl?.[STATE4];
    if (!state) return;
    const { wrapper, trigger, input } = state;
    trigger.removeEventListener?.("click", state.onTriggerClick);
    trigger.removeEventListener?.("keydown", state.onTriggerKey);
    input.removeEventListener?.("input", state.onInput);
    input.removeEventListener?.("keydown", state.onInputKey);
    selectEl.removeEventListener?.("change", state.onChange);
    state.doc?.removeEventListener?.("click", state.onDocClick, true);
    state.observer?.disconnect?.();
    unbindRows(state);
    wrapper.parentNode?.removeChild?.(wrapper);
    selectEl.classList.remove(SR_CLASS);
    delete selectEl[STATE4];
  }
  function searchPlaceholder(selectEl, doc) {
    const attr = selectEl.getAttribute?.("data-amarra-select-search-placeholder");
    if (attr) return attr;
    const lang = doc?.documentElement?.lang ?? "";
    return String(lang).toLowerCase().startsWith("pt") ? "Buscar\u2026" : "Search\u2026";
  }
  function normalize(text) {
    return String(text ?? "").normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().trim();
  }

  // pkg/amarra/js/hook_sidebar.mjs
  var OPEN_ATTR = "data-amarra-sidebar-open";
  var STATE5 = "_amarraSidebar";
  function makeSidebar(deps = {}) {
    const isDesktop = deps.isDesktop ?? (() => false);
    return {
      connect(el) {
        if (!el?.getAttribute) return;
        const doc = el.ownerDocument ?? deps.document ?? null;
        const sel = el.getAttribute("data-amarra-sidebar-target") || "";
        const panel = sel && doc?.querySelector ? doc.querySelector(sel) : null;
        if (!panel) return;
        const state = {};
        const apply = (next) => {
          state.open = next;
          if (next) panel.setAttribute?.(OPEN_ATTR, "");
          else panel.removeAttribute?.(OPEN_ATTR);
          el.setAttribute?.("aria-expanded", next ? "true" : "false");
        };
        const onToggle = () => apply(!state.open);
        const onKey2 = (ev) => {
          if (ev?.key !== "Escape" || !state.open) return;
          apply(false);
          el.focus?.();
        };
        const onDocClick = (ev) => {
          if (!state.open || isDesktop()) return;
          const target = ev?.target;
          if (el.contains?.(target) || panel.contains?.(target)) return;
          apply(false);
        };
        state.onToggle = onToggle;
        state.onKey = onKey2;
        state.onDocClick = onDocClick;
        el.addEventListener?.("click", onToggle);
        doc?.addEventListener?.("keydown", onKey2);
        doc?.addEventListener?.("click", onDocClick, true);
        apply(false);
        el[STATE5] = state;
      },
      disconnect(el) {
        const state = el?.[STATE5];
        if (!state) return;
        el.removeEventListener?.("click", state.onToggle);
        const doc = el.ownerDocument ?? deps.document ?? null;
        doc?.removeEventListener?.("keydown", state.onKey);
        doc?.removeEventListener?.("click", state.onDocClick, true);
        delete el[STATE5];
      }
    };
  }
  var sidebar = makeSidebar();

  // pkg/amarra/js/hook_theme.mjs
  var CLICK3 = "_amarraThemeClick";
  var DEFAULT_KEY = "amarra-theme";
  var DEFAULT_CLASS = "light";
  function makeTheme(opts = {}) {
    const getHtml = opts.html ?? (() => globalThis.document?.documentElement);
    const getStorage = () => opts.storage ?? globalThis.localStorage;
    const getMeta = opts.themeColorMeta ?? (() => globalThis.document?.querySelector?.('meta[name="theme-color"]'));
    const keyFor = (el) => el?.getAttribute?.("data-amarra-theme-key") || opts.key || DEFAULT_KEY;
    const classFor = (el) => el?.getAttribute?.("data-amarra-theme-class") || opts.className || DEFAULT_CLASS;
    function apply(on, el) {
      const className = classFor(el);
      const key = keyFor(el);
      const html = getHtml();
      if (html?.classList) {
        if (on) html.classList.add(className);
        else html.classList.remove(className);
      }
      try {
        getStorage()?.setItem?.(key, on ? className : "");
      } catch {
      }
      const meta = getMeta?.();
      const lightColor = el?.getAttribute?.("data-amarra-theme-color") || opts.lightColor;
      const darkColor = el?.getAttribute?.("data-amarra-theme-color-off") || opts.darkColor;
      const color = on ? lightColor : darkColor;
      if (meta && color) meta.setAttribute?.("content", color);
      const onLabel = el?.getAttribute?.("data-amarra-theme-on-label") || opts.onLabel;
      const offLabel = el?.getAttribute?.("data-amarra-theme-off-label") || opts.offLabel;
      const label = on ? onLabel : offLabel;
      if (label && el) swapThemeLabel(el, label);
      el?.setAttribute?.("aria-pressed", on ? "true" : "false");
    }
    return {
      connect(el) {
        if (!el || typeof el.addEventListener !== "function") return;
        let stored = "";
        try {
          stored = getStorage()?.getItem?.(keyFor(el)) ?? "";
        } catch {
          stored = "";
        }
        if (stored === classFor(el)) apply(true, el);
        const fn = (ev) => {
          ev?.preventDefault?.();
          const html = getHtml();
          const on = !html?.classList?.contains?.(classFor(el));
          apply(on, el);
        };
        el[CLICK3] = fn;
        el.addEventListener("click", fn);
      },
      disconnect(el) {
        const fn = el?.[CLICK3];
        if (!fn || typeof el.removeEventListener !== "function") return;
        el.removeEventListener("click", fn);
        delete el[CLICK3];
      }
    };
  }
  var theme = makeTheme();
  function swapThemeLabel(el, label) {
    const slot = el.querySelector?.("[data-amarra-theme-label]");
    if (slot) {
      slot.textContent = label;
      return;
    }
    if (el.children?.length) return;
    el.textContent = label;
  }

  // pkg/amarra/js/hook_tour.mjs
  var STATE6 = "_amarraTourState";
  var START = "_amarraTourStart";
  var PAD = 6;
  var SHADOW = "0 0 0 9999px rgba(0, 0, 0, 0.72)";
  var MIN_VISIBLE = 4;
  function makeTour(deps = {}) {
    const raf = deps.requestAnimationFrame ?? ((fn) => {
      const doc = deps.document;
      const win = doc?.defaultView;
      if (win?.requestAnimationFrame) return win.requestAnimationFrame(fn);
      return fn();
    });
    const api = {
      connect(el) {
        if (!el?.querySelectorAll) return;
        const doc = el.ownerDocument ?? deps.document ?? null;
        if (!doc) return;
        const starts = [...el.querySelectorAll("[data-amarra-tour-start]") ?? []];
        const state = { el, doc, starts, tour: null };
        for (const btn of starts) {
          const fn = (ev) => {
            ev?.preventDefault?.();
            begin(state, btn, raf);
          };
          btn[START] = fn;
          btn.addEventListener?.("click", fn);
        }
        el[STATE6] = state;
      },
      updated(el) {
        api.disconnect(el);
        api.connect(el);
      },
      disconnect(el) {
        const state = el?.[STATE6];
        if (!state) return;
        for (const btn of state.starts ?? []) {
          const fn = btn?.[START];
          if (!fn) continue;
          btn.removeEventListener?.("click", fn);
          delete btn[START];
        }
        end(state);
        delete el[STATE6];
      }
    };
    return api;
  }
  var tour = makeTour();
  function begin(state, opener, raf) {
    if (state.tour) return;
    const steps = collectSteps(state.el, state.doc);
    if (!steps.length) return;
    const t = {
      state,
      raf,
      steps,
      index: 0,
      ui: buildUI2(state.doc),
      opener,
      queued: false
    };
    t.onKey = (ev) => onKey(t, ev);
    t.onPlace = () => schedulePlace(t);
    bindButtons(t);
    state.doc.addEventListener?.("keydown", t.onKey, true);
    state.doc.defaultView?.addEventListener?.("scroll", t.onPlace, true);
    state.doc.defaultView?.addEventListener?.("resize", t.onPlace, true);
    state.tour = t;
    show(t, 0);
  }
  function end(state) {
    const t = state?.tour;
    if (!t) return;
    state.doc.removeEventListener?.("keydown", t.onKey);
    state.doc.defaultView?.removeEventListener?.("scroll", t.onPlace, true);
    state.doc.defaultView?.removeEventListener?.("resize", t.onPlace, true);
    for (const node of [t.ui.block, t.ui.hole, t.ui.tip]) {
      node?.parentNode?.removeChild?.(node);
    }
    state.tour = null;
    t.opener?.focus?.();
  }
  function bindButtons(t) {
    const { prevBtn, nextBtn, skipBtn } = t.ui;
    prevBtn.addEventListener?.("click", (ev) => {
      ev?.preventDefault?.();
      if (t.index > 0) show(t, t.index - 1);
    });
    nextBtn.addEventListener?.("click", (ev) => {
      ev?.preventDefault?.();
      if (t.index < t.steps.length - 1) show(t, t.index + 1);
      else end(t.state);
    });
    skipBtn.addEventListener?.("click", (ev) => {
      ev?.preventDefault?.();
      end(t.state);
    });
  }
  function show(t, index) {
    t.index = Math.max(0, Math.min(index, t.steps.length - 1));
    const step = t.steps[t.index];
    step.el.scrollIntoView?.({ block: "center", inline: "nearest" });
    t.ui.heading.textContent = step.copy.title;
    t.ui.body.textContent = step.copy.text;
    t.ui.count.textContent = `${t.index + 1} / ${t.steps.length}`;
    t.ui.prevBtn.disabled = t.index === 0;
    t.ui.nextBtn.textContent = t.index === t.steps.length - 1 ? "Concluir" : "Pr\xF3ximo \u2192";
    place(t);
    schedulePlace(t);
  }
  function schedulePlace(t) {
    if (t.queued) return;
    t.queued = true;
    t.raf(() => {
      t.queued = false;
      place(t);
    });
  }
  function place(t) {
    const step = t.steps[t.index];
    if (!step) return;
    const rect = step.el.getBoundingClientRect?.();
    if (!rect) return;
    const { hole, tip } = t.ui;
    hole.style.top = `${rect.top - PAD}px`;
    hole.style.left = `${rect.left - PAD}px`;
    hole.style.width = `${rect.width + PAD * 2}px`;
    hole.style.height = `${rect.height + PAD * 2}px`;
    const tipRect = tip.getBoundingClientRect?.() ?? { width: 0, height: 0 };
    const win = t.state.doc.defaultView;
    const vw = win?.innerWidth ?? 0;
    const vh = win?.innerHeight ?? 0;
    const below = rect.bottom + PAD + 14;
    const above = rect.top - PAD - 14 - tipRect.height;
    let top = below + tipRect.height + 12 <= vh ? below : above;
    let left = rect.left + rect.width / 2 - tipRect.width / 2;
    left = Math.max(12, Math.min(left, vw - tipRect.width - 12));
    top = Math.max(12, Math.min(top, vh - tipRect.height - 12));
    tip.style.top = `${top}px`;
    tip.style.left = `${left}px`;
  }
  function onKey(t, ev) {
    const key = ev?.key;
    if (key === "ArrowRight" || key === "Enter") {
      ev?.preventDefault?.();
      if (t.index < t.steps.length - 1) show(t, t.index + 1);
      else end(t.state);
    } else if (key === "ArrowLeft") {
      ev?.preventDefault?.();
      if (t.index > 0) show(t, t.index - 1);
    } else if (key === "Escape") {
      ev?.preventDefault?.();
      end(t.state);
    }
  }
  function collectSteps(el, doc) {
    const found = [...el.querySelectorAll?.("[data-amarra-tour-step]") ?? []];
    return found.filter((node) => isVisible(node, doc)).map((node) => ({ el: node, copy: copyFor(node) }));
  }
  function isVisible(node, doc) {
    const rect = node?.getBoundingClientRect?.();
    if (!rect) return false;
    if (rect.width <= MIN_VISIBLE || rect.height <= MIN_VISIBLE) return false;
    const style = doc?.defaultView?.getComputedStyle?.(node);
    if (style && (style.visibility === "hidden" || style.display === "none")) return false;
    return true;
  }
  function copyFor(node) {
    return {
      title: node.getAttribute?.("data-amarra-tour-title") ?? "",
      text: node.getAttribute?.("data-amarra-tour-text") ?? ""
    };
  }
  function buildUI2(doc) {
    const block = doc.createElement("div");
    block.setAttribute("data-amarra-tour-block", "");
    block.setAttribute("aria-hidden", "true");
    css(block, { position: "fixed", inset: "0", zIndex: "80" });
    const hole = doc.createElement("div");
    hole.setAttribute("data-amarra-tour-hole", "");
    css(hole, {
      position: "fixed",
      zIndex: "81",
      pointerEvents: "none",
      borderRadius: "0.5rem",
      boxShadow: SHADOW,
      outline: "2px solid currentColor",
      outlineOffset: "2px"
    });
    const tip = doc.createElement("div");
    tip.setAttribute("data-amarra-tour-tip", "");
    tip.setAttribute("role", "dialog");
    tip.setAttribute("aria-live", "polite");
    css(tip, {
      position: "fixed",
      zIndex: "82",
      width: "min(92vw, 23rem)",
      padding: "1rem",
      borderRadius: "0.75rem",
      background: "#0f172a",
      color: "#f8fafc",
      border: "1px solid rgba(148, 163, 184, 0.35)",
      boxShadow: "0 20px 40px -12px rgba(0, 0, 0, 0.5)",
      font: "14px/1.4 system-ui, sans-serif"
    });
    const header = doc.createElement("div");
    css(header, {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      gap: "0.5rem"
    });
    const count = doc.createElement("span");
    count.setAttribute("data-amarra-tour-count", "");
    css(count, { opacity: "0.7", fontSize: "0.75rem" });
    const skipBtn = tipButton(doc, "data-amarra-tour-skip", "pular", {});
    header.appendChild(count);
    header.appendChild(skipBtn);
    const heading = doc.createElement("h3");
    heading.setAttribute("data-amarra-tour-heading", "");
    css(heading, { margin: "0.5rem 0 0", fontSize: "1rem", fontWeight: "700" });
    const body = doc.createElement("p");
    body.setAttribute("data-amarra-tour-body", "");
    css(body, { margin: "0.25rem 0 0", opacity: "0.8" });
    const footer = doc.createElement("div");
    css(footer, {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      gap: "0.5rem",
      marginTop: "0.75rem"
    });
    const prevBtn = tipButton(doc, "data-amarra-tour-prev", "\u2190 Anterior", {});
    const nextBtn = tipButton(doc, "data-amarra-tour-next", "Pr\xF3ximo \u2192", {
      background: "#38bdf8",
      color: "#0f172a",
      borderColor: "transparent"
    });
    footer.appendChild(prevBtn);
    footer.appendChild(nextBtn);
    tip.appendChild(header);
    tip.appendChild(heading);
    tip.appendChild(body);
    tip.appendChild(footer);
    doc.body?.appendChild(block);
    doc.body?.appendChild(hole);
    doc.body?.appendChild(tip);
    return { block, hole, tip, count, heading, body, prevBtn, nextBtn, skipBtn };
  }
  function tipButton(doc, attr, label, extra) {
    const btn = doc.createElement("button");
    btn.setAttribute("type", "button");
    btn.setAttribute(attr, "");
    btn.textContent = label;
    css(btn, {
      cursor: "pointer",
      padding: "0.375rem 0.75rem",
      borderRadius: "0.5rem",
      border: "1px solid rgba(148, 163, 184, 0.5)",
      background: "transparent",
      color: "inherit",
      font: "inherit",
      fontWeight: "600",
      ...extra
    });
    return btn;
  }
  function css(node, props) {
    if (!node) return;
    if (!node.style) node.style = {};
    for (const [key, value] of Object.entries(props)) node.style[key] = value;
  }

  // pkg/amarra/js/hook.mjs
  register("bulk", bulk);
  register("clipboard", clipboard);
  register("dialog", dialog);
  register("dropdown", dropdown);
  register("nav", nav);
  register("password", password);
  register("reveal", reveal);
  register("select", select);
  register("sidebar", sidebar);
  register("theme", theme);
  register("tour", tour);
  registerAuto("select", "select");
  var ON_CLASSES = ["bg-green-50", "text-green-700"];
  var OFF_CLASSES = ["bg-slate-100", "text-slate-600"];
  var TOAST_MS = 2e3;
  function csrfTokenFromMeta(htmlOrDoc) {
    if (!htmlOrDoc) return "";
    if (typeof htmlOrDoc === "string") {
      const named = htmlOrDoc.match(/<meta\b[^>]*\bname\s*=\s*["']csrf-token["'][^>]*>/i);
      const tag = named?.[0] || htmlOrDoc.match(
        /<meta\b[^>]*\bcontent\s*=\s*["'][^"']*["'][^>]*\bname\s*=\s*["']csrf-token["'][^>]*>/i
      )?.[0];
      if (!tag) return "";
      const content = tag.match(/\bcontent\s*=\s*["']([^"']*)["']/i);
      return content ? content[1] : "";
    }
    const meta = htmlOrDoc.querySelector?.('meta[name="csrf-token"]');
    if (!meta) return "";
    return meta.content || meta.getAttribute?.("content") || "";
  }
  function showToast(message, doc, opts = {}) {
    if (!message || !doc) return;
    const host = doc.getElementById?.("amarra-toast-host");
    if (!host) return;
    if (host._amarraToastTimer) {
      clearTimeout(host._amarraToastTimer);
      host._amarraToastTimer = null;
    }
    host.innerHTML = '<div class="amarra-toast-enter fixed top-24 left-1/2 -translate-x-1/2 z-50 bg-slate-900 text-white px-5 py-3 rounded-2xl shadow-xl flex items-center gap-2 border border-slate-700/50" role="status"><span class="text-xs font-bold"></span></div>';
    const span = host.querySelector?.("span");
    if (span) span.textContent = message;
    const duration = opts.duration ?? TOAST_MS;
    if (duration > 0) {
      host._amarraToastTimer = setTimeout(() => {
        host.innerHTML = "";
        host._amarraToastTimer = null;
      }, duration);
    }
  }
  function applyFocus(selector, doc) {
    if (!selector || !doc?.querySelector) return;
    const el = doc.querySelector(selector);
    if (el && typeof el.focus === "function") el.focus();
  }
  function afterMorph(doc) {
    if (!doc?.querySelector) return;
    const marked = doc.querySelector("[data-amarra-focus]");
    if (!marked) return;
    const sel = marked.getAttribute?.("data-amarra-focus");
    if (sel && sel !== "true") {
      applyFocus(sel, doc);
      return;
    }
    if (typeof marked.focus === "function") marked.focus();
  }
  function applyOptimistic(el, mode) {
    if (!el) return null;
    if (mode === "count") {
      const countEl = el.querySelector?.("[data-amarra-count]") || el;
      const prev = countEl.textContent;
      const n = parseInt(String(prev ?? "").trim(), 10);
      countEl.textContent = String((Number.isNaN(n) ? 0 : n) + 1);
      return { el, mode, countEl, prev };
    }
    if (mode === "remove") {
      const hadOpacity = !!el.classList?.contains("opacity-0");
      el.classList?.add("opacity-0", "transition-opacity", "duration-150");
      return { el, mode, hadOpacity };
    }
    const wasOn = hasClasses(el, ON_CLASSES);
    if (wasOn) setClasses(el, OFF_CLASSES, ON_CLASSES);
    else setClasses(el, ON_CLASSES, OFF_CLASSES);
    return { el, mode: "toggle", wasOn };
  }
  function rollbackOptimistic(state) {
    if (!state?.el) return;
    const { el } = state;
    if (state.mode === "count") {
      state.countEl.textContent = state.prev;
      return;
    }
    if (state.mode === "remove") {
      if (!state.hadOpacity) el.classList?.remove("opacity-0", "transition-opacity", "duration-150");
      return;
    }
    if (state.wasOn) setClasses(el, ON_CLASSES, OFF_CLASSES);
    else setClasses(el, OFF_CLASSES, ON_CLASSES);
  }
  function start(opts = {}) {
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    if (!doc || typeof doc.addEventListener !== "function") return;
    if (doc.documentElement?.dataset?.amarraHook === "true") return;
    if (doc.documentElement?.dataset) doc.documentElement.dataset.amarraHook = "true";
    register("bulk", bulk);
    register("clipboard", clipboard);
    register("dialog", dialog);
    register("dropdown", dropdown);
    register("nav", nav);
    register("password", password);
    register("reveal", reveal);
    register("select", select);
    register("sidebar", sidebar);
    register("theme", theme);
    register("tour", tour);
    registerAuto("select", "select");
    scan(doc);
    let optimistic = null;
    doc.addEventListener("amarra:toast", (ev) => {
      showToast(ev.detail?.message ?? "", doc, opts);
    });
    doc.addEventListener("amarra:morphed", () => {
      optimistic = null;
      afterMorph(doc);
      scan(doc);
    });
    doc.addEventListener("amarra:drive-error", (ev) => {
      rollbackOptimistic(optimistic);
      optimistic = null;
      const message = ev.detail?.message;
      if (message) showToast(message, doc);
    });
    doc.addEventListener(
      "click",
      (ev) => {
        const target = ev.target?.closest?.("[data-amarra-optimistic]");
        if (!target) return;
        optimistic = applyOptimistic(target, target.getAttribute("data-amarra-optimistic"));
        target.setAttribute?.("aria-busy", "true");
      },
      true
    );
  }
  function hasClasses(el, classes2) {
    return classes2.every((c) => el.classList?.contains(c));
  }
  function setClasses(el, add, remove) {
    remove.forEach((c) => el.classList?.remove(c));
    add.forEach((c) => el.classList?.add(c));
  }

  // pkg/amarra/js/frame.mjs
  var frame_exports = {};
  __export(frame_exports, {
    define: () => define,
    frameHeaders: () => frameHeaders,
    frameTarget: () => frameTarget,
    loadFrame: () => loadFrame,
    observeLazy: () => observeLazy,
    visitIntoFrame: () => visitIntoFrame
  });
  function frameHeaders(id, csrfToken) {
    const headers = {
      "Amarra-Frame": String(id ?? ""),
      Accept: "text/html"
    };
    if (csrfToken) headers["X-CSRF-Token"] = csrfToken;
    return headers;
  }
  function frameTarget(el) {
    return el?.getAttribute?.("data-amarra-frame") || "";
  }
  async function visitIntoFrame(el, href, opts = {}) {
    const id = frameTarget(el);
    if (!id || id === "_top") return false;
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    const frame = doc?.getElementById?.(id) || null;
    if (!frame) return false;
    const prev = frame.getAttribute?.("src");
    if (typeof frame.setAttribute === "function") frame.setAttribute("src", href);
    const tag = frame.tagName ? String(frame.tagName).toUpperCase() : "";
    if (tag === "AMARRA-FRAME" && prev !== href) return true;
    await loadFrame(frame, { ...opts, document: doc });
    return true;
  }
  function observeLazy(el, opts = {}) {
    const IO = opts.IntersectionObserver ?? globalThis.IntersectionObserver;
    if (typeof IO !== "function") {
      return loadFrame(el, opts);
    }
    const io = new IO((entries) => {
      if (!entries?.some?.((e) => e.isIntersecting)) return;
      io.disconnect();
      void loadFrame(el, opts);
    });
    io.observe(el);
    return io;
  }
  async function loadFrame(el, opts = {}) {
    if (!el) return;
    const src = el.getAttribute?.("src");
    if (!src) return;
    const id = el.getAttribute?.("id") || el.id || "";
    const fetchFn = opts.fetchFn ?? opts.fetch ?? fetch;
    const csrfToken = opts.csrfToken ?? csrfTokenFromMeta(opts.document ?? (typeof document !== "undefined" ? document : ""));
    const seq = bumpSequence(el);
    const res = await fetchFn(src, {
      headers: frameHeaders(id, csrfToken),
      credentials: "same-origin",
      redirect: "follow"
    });
    const html = await res.text();
    if (!isCurrentSequence(el, seq)) return;
    if (el.isConnected === false) return;
    (opts.morphFn ?? morph)(el, html);
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    if (doc && typeof doc.dispatchEvent === "function") {
      doc.dispatchEvent(new CustomEvent("amarra:morphed", { bubbles: true }));
    }
    if (el.hasAttribute?.("amarra-push") && opts.history?.pushState) {
      opts.history.pushState({ amarra: true }, "", src);
    }
  }
  function define(opts = {}) {
    const registry = opts.customElements ?? (typeof customElements !== "undefined" ? customElements : null);
    const Base = opts.HTMLElement ?? (typeof HTMLElement !== "undefined" ? HTMLElement : null);
    if (!registry || !Base || typeof registry.define !== "function") return false;
    if (typeof registry.get === "function" && registry.get("amarra-frame")) return true;
    class AmarraFrame extends Base {
      connectedCallback() {
        if (!this.getAttribute("src")) return;
        if (this.getAttribute("loading") === "lazy") {
          observeLazy(this, opts);
          return;
        }
        loadFrame(this, opts);
      }
      static get observedAttributes() {
        return ["src"];
      }
      attributeChangedCallback(name, prev, next) {
        if (name === "src" && next && prev !== next) loadFrame(this, opts);
      }
    }
    registry.define("amarra-frame", AmarraFrame);
    return true;
  }

  // pkg/amarra/js/stream.mjs
  var stream_exports = {};
  __export(stream_exports, {
    applyOp: () => applyOp,
    connect: () => connect,
    isStreamResponse: () => isStreamResponse,
    parseSSE: () => parseSSE,
    start: () => start2
  });
  var STREAM_KINDS = [
    "append",
    "prepend",
    "replace",
    "morph",
    "remove",
    "toast",
    "before",
    "after"
  ];
  function parseSSE(chunk) {
    const events = [];
    const text = String(chunk ?? "").replace(/\r\n?/g, "\n");
    for (const block of text.split("\n\n")) {
      if (!block.trim()) continue;
      let kind = "message";
      let target;
      const dataLines = [];
      for (const line of block.split("\n")) {
        if (!line || line.startsWith(":")) continue;
        if (line.startsWith("event:")) {
          kind = line.slice(6).trim();
          continue;
        }
        if (line.startsWith("id:")) {
          const rest = line.slice(3);
          target = rest.startsWith(" ") ? rest.slice(1) : rest;
          continue;
        }
        if (line.startsWith("data:")) {
          const rest = line.slice(5);
          dataLines.push(rest.startsWith(" ") ? rest.slice(1) : rest);
        }
      }
      const op = { kind, html: dataLines.join("\n") };
      if (target) op.target = target;
      events.push(op);
    }
    return events;
  }
  function applyOp(op, doc, opts = {}) {
    if (!op) return;
    if (op.kind === "toast") {
      if (doc && typeof doc.dispatchEvent === "function") {
        doc.dispatchEvent(
          new CustomEvent("amarra:toast", { bubbles: true, detail: { message: op.html ?? "" } })
        );
      }
      return;
    }
    const id = op.target || opts.defaultTarget;
    const el = id && doc?.getElementById ? doc.getElementById(id) : null;
    if (!el) return;
    const html = op.html ?? "";
    switch (op.kind) {
      case "append":
        insertHTML(el, "beforeend", html);
        break;
      case "prepend":
        insertHTML(el, "afterbegin", html);
        break;
      case "before":
        insertHTML(el, "beforebegin", html);
        break;
      case "after":
        insertHTML(el, "afterend", html);
        break;
      case "replace":
        el.outerHTML = html;
        break;
      case "morph":
        (opts.morphFn ?? morph)(el, html);
        break;
      case "remove":
        el.remove?.();
        break;
    }
  }
  function connect(url, opts = {}) {
    const ES = opts.EventSource ?? (typeof EventSource !== "undefined" ? EventSource : null);
    if (!ES || !url) return null;
    const src = new ES(url);
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    for (const kind of STREAM_KINDS) {
      src.addEventListener(kind, (ev) => {
        for (const op of parseSSE(sseEnvelope(kind, ev.data))) {
          applyOp(op, doc, opts);
        }
      });
    }
    return src;
  }
  function start2(opts = {}) {
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    if (!doc || typeof doc.addEventListener !== "function") return;
    if (doc.documentElement?.dataset?.amarraStream === "true") return;
    if (doc.documentElement?.dataset) doc.documentElement.dataset.amarraStream = "true";
    const sources = /* @__PURE__ */ new Map();
    const sync = () => {
      for (const [el, src] of sources) {
        if (el.isConnected === false) {
          src?.close?.();
          sources.delete(el);
        }
      }
      const nodes = typeof doc.querySelectorAll === "function" ? doc.querySelectorAll("[data-amarra-stream]") : [];
      for (const el of nodes) {
        if (el === doc.documentElement) continue;
        if (sources.has(el)) continue;
        const url = el.getAttribute?.("data-amarra-stream");
        if (!url) continue;
        const src = connect(url, {
          ...opts,
          document: doc,
          defaultTarget: el.getAttribute?.("data-amarra-target") || opts.defaultTarget
        });
        if (src) sources.set(el, src);
      }
    };
    sync();
    doc.addEventListener("amarra:morphed", sync);
  }
  function sseEnvelope(kind, data) {
    const lines = String(data ?? "").split("\n").map((line) => `data: ${line}`);
    return `event: ${kind}
${lines.join("\n")}

`;
  }
  function isStreamResponse(headers) {
    if (!headers) return false;
    const ct = typeof headers.get === "function" ? headers.get("content-type") : headers["content-type"] || headers["Content-Type"];
    return String(ct || "").includes("vnd.amarra-stream");
  }
  function insertHTML(el, pos, html) {
    if (typeof el.insertAdjacentHTML === "function") {
      el.insertAdjacentHTML(pos, html);
      return;
    }
    if (pos === "afterbegin") el.innerHTML = html + (el.innerHTML || "");
    else el.innerHTML = (el.innerHTML || "") + html;
  }

  // pkg/amarra/js/drive_head.mjs
  function extractTitle(html) {
    const m = String(html ?? "").match(/<title\b[^>]*>([\s\S]*?)<\/title>/i);
    return m ? m[1].trim() : null;
  }
  function extractHTMLAttr(html, name) {
    return extractTagAttr(html, "html", name);
  }
  function extractBodyAttr(html, name) {
    return extractTagAttr(html, "body", name);
  }
  function extractTagAttr(html, tag, name) {
    const open2 = String(html ?? "").match(new RegExp(`<${tag}\\b[^>]*>`, "i"))?.[0] ?? "";
    const escaped = String(name).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const m = open2.match(new RegExp(`\\s${escaped}\\s*=\\s*["']([^"']*)["']`, "i"));
    return m ? m[1] : null;
  }
  function applyHead(doc, html) {
    if (!doc) return;
    const title = extractTitle(html);
    if (title != null) doc.title = title;
    const lang = extractHTMLAttr(html, "lang");
    if (lang != null && doc.documentElement) doc.documentElement.lang = lang;
    const token = csrfTokenFromMeta(html);
    if (!token) return;
    const meta = doc.querySelector?.('meta[name="csrf-token"]');
    if (!meta) return;
    if (typeof meta.setAttribute === "function") meta.setAttribute("content", token);
    else meta.content = token;
  }
  function ensureVeilKeyframes(doc) {
    if (!doc?.createElement || doc.getElementById?.("amarra-veil-style")) return;
    const style = doc.createElement("style");
    style.id = "amarra-veil-style";
    style.textContent = "@keyframes amarra-spin{to{transform:rotate(360deg)}}@media (prefers-reduced-motion:reduce){#amarra-veil div{animation:none!important}}";
    (doc.head ?? doc.body)?.appendChild?.(style);
  }
  function showProgress(doc) {
    if (!doc?.createElement || !doc.body) return;
    let bar = doc.getElementById?.("amarra-progress");
    if (!bar) {
      bar = doc.createElement("div");
      bar.id = "amarra-progress";
      bar.setAttribute("role", "progressbar");
      bar.style.cssText = "position:fixed;top:0;left:0;right:0;height:2px;background:#1f8ba3;z-index:9999";
      doc.body.appendChild(bar);
    }
    bar.hidden = false;
    let veil = doc.getElementById?.("amarra-veil");
    if (!veil) {
      veil = doc.createElement("div");
      veil.id = "amarra-veil";
      veil.setAttribute("role", "status");
      veil.setAttribute("aria-label", "Loading");
      veil.style.cssText = "position:fixed;inset:0;z-index:9998;display:flex;align-items:center;justify-content:center;background:rgba(8,19,24,.45);backdrop-filter:blur(2px);opacity:0;transition:opacity .18s ease;pointer-events:none";
      const spinner = doc.createElement("div");
      spinner.setAttribute("role", "presentation");
      spinner.style.cssText = "width:52px;height:52px;border-radius:50%;border:4px solid rgba(31,139,163,.25);border-top-color:#1f8ba3;animation:amarra-spin .9s linear infinite";
      veil.appendChild?.(spinner);
      ensureVeilKeyframes(doc);
      doc.body.appendChild(veil);
    }
    veil.hidden = false;
    if (veil.style) veil.style.opacity = "1";
  }
  function hideProgress(doc) {
    const bar = doc?.getElementById?.("amarra-progress");
    if (bar) bar.hidden = true;
    const veil = doc?.getElementById?.("amarra-veil");
    if (veil) {
      if (veil.style) veil.style.opacity = "0";
      veil.hidden = true;
    }
  }

  // pkg/amarra/js/drive_form.mjs
  function confirmOk(el, confirmFn) {
    const msg = el?.getAttribute?.("data-amarra-confirm");
    if (!msg) return true;
    const fn = confirmFn ?? (typeof globalThis.confirm === "function" ? globalThis.confirm.bind(globalThis) : () => true);
    return !!fn(msg);
  }
  function requestMethod(el, fallback2 = "GET") {
    const attr = el?.getAttribute?.("data-amarra-method");
    if (attr) return String(attr).toUpperCase();
    const hidden = el?.querySelector?.('input[name="_method"]');
    if (hidden?.value) return String(hidden.value).toUpperCase();
    return String(fallback2 || "GET").toUpperCase();
  }
  function disableSubmit(el) {
    if (!el) return null;
    const prev = { el, disabled: !!el.disabled, text: el.textContent };
    el.disabled = true;
    const withText = el.getAttribute?.("data-amarra-disable-with");
    if (withText) el.textContent = withText;
    return prev;
  }
  function restoreSubmit(prev) {
    if (!prev?.el) return;
    prev.el.disabled = prev.disabled;
    if (prev.text != null) prev.el.textContent = prev.text;
  }
  function withQuery(action, fd) {
    if (!fd || typeof fd.entries !== "function") return action;
    const params = new URLSearchParams();
    for (const [key, value] of fd.entries()) {
      if (typeof value === "string") params.append(key, value);
    }
    const raw = String(action ?? "");
    try {
      const u = new URL(raw, "http://amarra.invalid");
      u.search = params.toString();
      if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(raw)) {
        return u.toString();
      }
      return u.pathname + u.search + u.hash;
    } catch {
      const q = params.toString();
      if (!q) return raw;
      const hashAt = raw.indexOf("#");
      const before = hashAt === -1 ? raw : raw.slice(0, hashAt);
      const hash = hashAt === -1 ? "" : raw.slice(hashAt);
      const base = before.split("?")[0];
      return `${base}?${q}${hash}`;
    }
  }
  function driveFormBody(formData, URLSearchParamsCtor = URLSearchParams, FileCtor = typeof File !== "undefined" ? File : null) {
    if (!formData) return null;
    for (const [, value] of formData.entries()) {
      if (FileCtor && value instanceof FileCtor) return formData;
    }
    const body = new URLSearchParamsCtor();
    for (const [key, value] of formData.entries()) body.append(key, value);
    return body;
  }

  // pkg/amarra/js/drive_restore.mjs
  function captureScroll(history, y) {
    if (!history?.replaceState) return;
    const prev = history.state && typeof history.state === "object" ? history.state : {};
    history.replaceState({ ...prev, amarra: true, scrollY: y ?? 0 }, "");
  }
  function restoreScroll(win, state) {
    const y = state?.scrollY;
    if (typeof y !== "number") return;
    win?.scrollTo?.(0, y);
  }
  function focusFirstInvalid(doc) {
    const el = doc?.querySelector?.('[aria-invalid="true"], [data-amarra-invalid]');
    if (el && typeof el.focus === "function") el.focus();
  }

  // pkg/amarra/js/drive_download.mjs
  function isMutationMethod(method) {
    const verb = String(method ?? "GET").toUpperCase();
    return verb !== "GET" && verb !== "HEAD";
  }
  function filenameFromDisposition(disposition, requestURL = "") {
    const raw = String(disposition ?? "");
    const extended = raw.match(/filename\*\s*=\s*([^;]+)/i);
    if (extended) return sanitizeFilename(decodeFilename(extendedFilename(extended[1])));
    const quoted = raw.match(/filename\s*=\s*"([^"]*)"/i);
    if (quoted) return sanitizeFilename(decodeFilename(quoted[1]));
    const plain = raw.match(/filename\s*=\s*([^;]+)/i);
    if (plain) return sanitizeFilename(decodeFilename(plain[1].trim()));
    return filenameFromURL(requestURL);
  }
  function extendedFilename(value) {
    const name = value.trim().replace(/^"|"$/g, "");
    const at = name.indexOf("''");
    return at >= 0 ? name.slice(at + 2) : name;
  }
  function decodeFilename(value) {
    try {
      return decodeURIComponent(value);
    } catch {
      return value;
    }
  }
  function filenameFromURL(url) {
    try {
      const path = new URL(String(url || ""), "http://amarra.invalid").pathname;
      return sanitizeFilename(path.slice(path.lastIndexOf("/") + 1));
    } catch {
      return "download";
    }
  }
  function sanitizeFilename(value) {
    const clean = String(value ?? "").replace(/[\u0000-\u001f\\/]+/g, "_").replace(/^\.+/, "").trim();
    return clean || "download";
  }
  async function downloadBlobResponse(res, opts = {}) {
    const blob = await res.blob();
    if (typeof opts.isCurrent === "function" && !opts.isCurrent()) return null;
    const filename = filenameFromDisposition(
      res.headers?.get?.("content-disposition"),
      res.url || opts.url
    );
    const createObjectURL = opts.createObjectURL ?? globalThis.URL?.createObjectURL?.bind(globalThis.URL);
    const revokeObjectURL = opts.revokeObjectURL ?? globalThis.URL?.revokeObjectURL?.bind(globalThis.URL);
    if (typeof createObjectURL !== "function") {
      throw new Error("download: URL.createObjectURL is unavailable");
    }
    const objectURL = createObjectURL(blob);
    const anchor = downloadAnchor(opts.document, objectURL, filename);
    anchor.click?.();
    anchor.remove?.();
    scheduleRevoke(objectURL, revokeObjectURL, opts.schedule);
    return { filename };
  }
  function downloadAnchor(doc, objectURL, filename) {
    if (!doc?.createElement) throw new Error("download: document.createElement is unavailable");
    const anchor = doc.createElement("a");
    anchor.href = objectURL;
    anchor.download = filename;
    anchor.rel = "noopener";
    anchor.hidden = true;
    (doc.body ?? doc.documentElement)?.appendChild?.(anchor);
    return anchor;
  }
  function scheduleRevoke(objectURL, revokeObjectURL, schedule) {
    if (typeof revokeObjectURL !== "function") return;
    const later = schedule ?? globalThis.setTimeout?.bind(globalThis);
    const revoke = () => revokeObjectURL(objectURL);
    if (typeof later === "function") later(revoke, 0);
    else revoke();
  }

  // pkg/amarra/js/drive.mjs
  function shouldInterceptClick({
    href,
    target,
    download,
    origin,
    locationOrigin,
    skip,
    currentHref,
    resolvedHref,
    button
  } = {}) {
    if (skip) return false;
    if (button != null && button !== 0) return false;
    if (download) return false;
    if (target && target !== "_self") return false;
    if (!href) return false;
    if (/^(mailto|javascript|tel):/i.test(href)) return false;
    if (isHashOnlyNavigation(href, resolvedHref, currentHref)) return false;
    if (origin && locationOrigin && origin !== locationOrigin) return false;
    return true;
  }
  function shouldInterceptSubmit({ skip, target, method, origin, locationOrigin } = {}) {
    if (skip) return false;
    if (target && target !== "_self") return false;
    if (String(method || "GET").toLowerCase() === "dialog") return false;
    if (origin && locationOrigin && origin !== locationOrigin) return false;
    return true;
  }
  function driveHeaders(csrfToken) {
    const headers = {
      "Amarra-Drive": "true",
      Accept: "text/html"
    };
    if (csrfToken) headers["X-CSRF-Token"] = csrfToken;
    return headers;
  }
  function extractMainHTML(html) {
    const str = String(html ?? "");
    const open2 = extractMainOpen(str);
    if (!open2) return null;
    const start5 = open2.index + open2[0].length;
    return sliceMatchingClose(str, start5, open2[1]);
  }
  function extractMainTagName(html) {
    return extractMainOpen(String(html ?? ""))?.[1]?.toUpperCase() ?? null;
  }
  function defaultDriveWarn(msg) {
    try {
      globalThis.console?.warn?.(msg);
    } catch {
    }
  }
  var DRIVE_ERROR_MESSAGE = "N\xE3o foi poss\xEDvel carregar a p\xE1gina.";
  function applyDriveResponse({
    status,
    html,
    url,
    main,
    morphFn,
    location,
    history,
    document: doc,
    push = true,
    window: win,
    warn = defaultDriveWarn
  } = {}) {
    if (status === 401 || status === 403) {
      location?.reload?.();
      return { action: "reload" };
    }
    const fragment = extractMainHTML(html);
    if (fragment == null) {
      if (status !== 200 && status !== 422) {
        emitDriveError(doc, { status, message: DRIVE_ERROR_MESSAGE });
        return { action: "ignore" };
      }
      warn(
        `amarra drive: ignored ${status} response for ${url ?? "(unknown url)"} \u2014 #amarra-main missing or unbalanced HTML`
      );
      return { action: "ignore" };
    }
    if (needsFullVisit({ html, main, document: doc })) {
      assignLocation(location, url);
      return { action: "assign" };
    }
    applyHead(doc, html);
    applyLayoutMarker(doc, html);
    applyShellMarker(doc, html);
    if (main) (morphFn ?? morph)(main, fragment);
    if ((status === 200 || status === 404) && push && url && history?.pushState) {
      if (!location?.href || url !== location.href) {
        history.pushState({ amarra: true, scrollY: 0 }, "", url);
      }
      scrollToFragment(doc, win, url);
    }
    if (status === 200 && !push) restoreScroll(win, history?.state);
    if (status === 422) focusFirstInvalid(doc);
    if (doc && typeof doc.dispatchEvent === "function") {
      doc.dispatchEvent(new CustomEvent("amarra:morphed", { bubbles: true }));
    }
    return { action: "morph" };
  }
  function needsFullVisit({ html, main, document: doc }) {
    if (markerChanged(
      doc?.documentElement?.dataset?.amarraLayout,
      extractHTMLAttr(html, "data-amarra-layout")
    )) {
      return true;
    }
    if (markerChanged(doc?.body?.dataset?.amarraShell, extractBodyAttr(html, "data-amarra-shell"))) {
      return true;
    }
    const currentTag = main?.tagName?.toUpperCase?.();
    const nextTag = extractMainTagName(html);
    return !!(currentTag && nextTag && currentTag !== nextTag);
  }
  function markerChanged(current, next) {
    return !!(current && next && current !== next);
  }
  function scrollToFragment(doc, win, url) {
    const target = fragmentTarget(doc, url);
    if (target && typeof target.scrollIntoView === "function") {
      target.scrollIntoView();
      return;
    }
    win?.scrollTo?.(0, 0);
  }
  function fragmentTarget(doc, url) {
    const id = decodeFragment(hashOf(url));
    if (!id || typeof doc?.getElementById !== "function") return null;
    return doc.getElementById(id);
  }
  function decodeFragment(hash) {
    const raw = hash.startsWith("#") ? hash.slice(1) : hash;
    if (!raw) return "";
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }
  function hashOf(url) {
    const str = String(url ?? "");
    const at = str.indexOf("#");
    return at === -1 ? "" : str.slice(at);
  }
  function withRequestHash(responseURL, requestURL) {
    const hash = hashOf(requestURL);
    if (!hash) return responseURL;
    const base = String(responseURL ?? "");
    const at = base.indexOf("#");
    return (at === -1 ? base : base.slice(0, at)) + hash;
  }
  function assignLocation(location, url) {
    if (url && typeof location?.assign === "function") {
      location.assign(url);
      return;
    }
    if (url && location) location.href = url;
  }
  function isHtmlResponse(headers) {
    const type = String(headers?.get?.("content-type") ?? "").split(";")[0].trim().toLowerCase();
    if (/attachment/i.test(String(headers?.get?.("content-disposition") ?? ""))) return false;
    return type === "" || type === "text/html" || type === "application/xhtml+xml";
  }
  function applyLayoutMarker(doc, html) {
    const layout = extractHTMLAttr(html, "data-amarra-layout");
    if (layout != null && doc?.documentElement?.dataset) {
      doc.documentElement.dataset.amarraLayout = layout;
    }
  }
  function applyShellMarker(doc, html) {
    const shell = extractBodyAttr(html, "data-amarra-shell");
    if (shell != null && doc?.body?.dataset) {
      doc.body.dataset.amarraShell = shell;
    }
  }
  async function visit(url, opts = {}) {
    const fetchFn = opts.fetchFn ?? opts.fetch ?? fetch;
    const doc = opts.document;
    const method = opts.method ?? "GET";
    const seq = bumpSequence(doc);
    showProgress(doc);
    try {
      const res = await fetchFn(url, {
        method,
        headers: { ...driveHeaders(opts.csrfToken), ...opts.headers },
        body: opts.body,
        redirect: "follow",
        credentials: "same-origin"
      });
      if (!isCurrentSequence(doc, seq)) {
        return { action: "superseded" };
      }
      const win = opts.window ?? (typeof window !== "undefined" ? window : null);
      const location = opts.location ?? win?.location ?? null;
      const history = opts.history ?? win?.history ?? null;
      if (opts.push !== false) captureScroll(history, win?.scrollY ?? 0);
      const authFailure = res.status === 401 || res.status === 403;
      const stream = isStreamResponse(res.headers);
      if (!authFailure && !stream && !isHtmlResponse(res.headers)) {
        if (isMutationMethod(method)) {
          const file = await downloadBlobResponse(res, {
            document: doc,
            url: res.url || url,
            createObjectURL: opts.createObjectURL,
            revokeObjectURL: opts.revokeObjectURL,
            schedule: opts.schedule,
            isCurrent: () => isCurrentSequence(doc, seq)
          });
          if (!file) return { action: "superseded" };
          return { action: "download", filename: file.filename };
        }
        assignLocation(location, res.url || url);
        return { action: "assign" };
      }
      const html = authFailure ? "" : await res.text();
      if (!isCurrentSequence(doc, seq)) return { action: "superseded" };
      if (stream) {
        for (const op of parseSSE(html)) applyOp(op, doc, opts);
        return { action: "stream" };
      }
      return applyDriveResponse({
        status: res.status,
        html,
        url: withRequestHash(res.url || url, url),
        main: doc?.querySelector?.("#amarra-main") ?? opts.main ?? null,
        morphFn: opts.morphFn,
        location,
        history,
        document: doc,
        push: opts.push !== false,
        window: win
      });
    } catch {
      emitDriveError(doc, { message: DRIVE_ERROR_MESSAGE });
      return { action: "error" };
    } finally {
      if (isCurrentSequence(doc, seq)) hideProgress(doc);
    }
  }
  function start3(opts = {}) {
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    if (!doc || typeof doc.addEventListener !== "function") return;
    if (doc.documentElement?.dataset?.amarraDrive === "true") return;
    if (doc.documentElement?.dataset) doc.documentElement.dataset.amarraDrive = "true";
    const location = opts.location ?? (typeof window !== "undefined" ? window.location : null);
    const history = opts.history ?? (typeof window !== "undefined" ? window.history : null);
    const fetchFn = opts.fetchFn ?? opts.fetch ?? (typeof fetch !== "undefined" ? fetch : null);
    const csrfToken = opts.csrfToken ?? csrfTokenFromMeta(doc);
    const FormDataCtor = opts.FormData ?? (typeof FormData !== "undefined" ? FormData : null);
    const shared = { ...opts, document: doc, location, history, fetchFn, csrfToken };
    doc.addEventListener("click", (event) => {
      if (event.defaultPrevented) return;
      if (event.button != null && event.button !== 0) return;
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const a = findAnchor(event.target);
      if (!a) return;
      const href = a.getAttribute?.("href") ?? a.href;
      const resolved = resolveURL(href, location);
      if (!shouldInterceptClick({
        href,
        resolvedHref: resolved?.href,
        currentHref: location?.href,
        target: a.getAttribute?.("target") ?? a.target ?? "",
        download: !!(a.hasAttribute?.("download") || a.download),
        origin: resolved?.origin,
        locationOrigin: location?.origin,
        skip: a.hasAttribute?.("data-amarra-skip"),
        button: event.button
      })) {
        return;
      }
      event.preventDefault();
      if (!confirmOk(a, opts.confirm)) return;
      const method = requestMethod(a, "GET");
      const hrefURL = resolved?.href ?? href;
      void (async () => {
        if (await visitIntoFrame(a, hrefURL, shared)) return;
        await visit(hrefURL, { ...shared, method });
      })().catch(() => emitDriveError(doc, { message: DRIVE_ERROR_MESSAGE }));
    });
    doc.addEventListener("submit", (event) => {
      if (event.defaultPrevented) return;
      const form = findForm(event.target);
      if (!form) return;
      const submitter = event.submitter;
      const rawAction = submitter?.getAttribute?.("formaction") || form.getAttribute?.("action") || form.action || location?.href || "";
      const method = (submitter?.getAttribute?.("formmethod") || form.getAttribute?.("method") || form.method || "GET").toUpperCase();
      const resolved = resolveURL(rawAction, location);
      if (!shouldInterceptSubmit({
        skip: form.hasAttribute?.("data-amarra-skip"),
        target: form.getAttribute?.("target") ?? form.target ?? "",
        method,
        origin: resolved?.origin,
        locationOrigin: location?.origin
      })) {
        return;
      }
      event.preventDefault();
      if (!confirmOk(form, opts.confirm) || !confirmOk(submitter, opts.confirm)) return;
      const verb = requestMethod(form, method);
      const fd = FormDataCtor ? formDataWithSubmitter(form, submitter, FormDataCtor) : null;
      const url = verb === "GET" ? withQuery(rawAction, fd) : rawAction;
      const body = verb === "GET" ? void 0 : driveFormBody(fd);
      const disabled = disableSubmit(submitter);
      void visit(url, {
        ...shared,
        method: verb,
        body
      }).catch(() => emitDriveError(doc, { message: DRIVE_ERROR_MESSAGE })).finally(() => restoreSubmit(disabled));
    });
    if (typeof window !== "undefined" && opts.popstate !== false) {
      window.addEventListener("popstate", () => {
        void visit(location?.href ?? window.location.href, { ...shared, push: false });
      });
    }
  }
  function isHashOnlyNavigation(href, resolvedHref, currentHref) {
    if (href === "#" || typeof href === "string" && href.startsWith("#")) return true;
    if (!currentHref) return false;
    try {
      const next = new URL(resolvedHref || href, currentHref);
      const cur = new URL(currentHref);
      if (next.origin !== cur.origin || next.pathname !== cur.pathname || next.search !== cur.search) {
        return false;
      }
      return next.hash !== cur.hash;
    } catch {
      return false;
    }
  }
  function sliceMatchingClose(str, start5, tag) {
    const lower = str.toLowerCase();
    const name = tag.toLowerCase();
    const closeToken = `</${name}>`;
    let depth = 1;
    let i = start5;
    while (i < str.length && depth > 0) {
      const nextOpen = findOpenTag(lower, i, name);
      const nextClose = lower.indexOf(closeToken, i);
      if (nextClose === -1) return null;
      if (nextOpen !== -1 && nextOpen < nextClose) {
        depth += 1;
        i = nextOpen + name.length + 1;
        continue;
      }
      depth -= 1;
      if (depth === 0) return str.slice(start5, nextClose);
      i = nextClose + closeToken.length;
    }
    return null;
  }
  function extractMainOpen(str) {
    return str.match(/<([a-zA-Z][\w:-]*)(?=[^>]*\sid\s*=\s*["']amarra-main["'])[^>]*>/i);
  }
  function findOpenTag(lower, from, name) {
    const token = `<${name}`;
    let i = from;
    while (i < lower.length) {
      const j = lower.indexOf(token, i);
      if (j === -1) return -1;
      const after = lower[j + token.length];
      if (after === ">" || after === "/" || after && /\s/.test(after)) return j;
      i = j + token.length;
    }
    return -1;
  }
  function formDataWithSubmitter(form, submitter, Ctor) {
    try {
      return new Ctor(form, submitter);
    } catch {
      const fd = new Ctor();
      if (submitter?.name && typeof fd.append === "function") {
        fd.append(submitter.name, submitter.value ?? "");
      }
      return fd;
    }
  }
  function emitDriveError(doc, detail) {
    if (doc && typeof doc.dispatchEvent === "function") {
      doc.dispatchEvent(
        new CustomEvent("amarra:drive-error", { bubbles: true, detail: detail ?? {} })
      );
    }
  }
  function findAnchor(target) {
    if (!target) return null;
    if (typeof target.closest === "function") return target.closest("a[href]");
    let node = target;
    while (node) {
      const tag = node.tagName;
      if ((tag === "A" || tag === "a") && (node.href || node.getAttribute?.("href"))) return node;
      node = node.parentElement || node.parentNode;
    }
    return null;
  }
  function findForm(target) {
    if (!target) return null;
    if (target.tagName === "FORM" || target.tagName === "form") return target;
    if (typeof target.closest === "function") return target.closest("form");
    return null;
  }
  function resolveURL(href, location) {
    if (href == null || href === "") {
      try {
        return location?.href ? new URL(location.href) : null;
      } catch {
        return null;
      }
    }
    try {
      return new URL(href, location?.href ?? location?.origin ?? "http://localhost");
    } catch {
      return null;
    }
  }

  // pkg/amarra/js/live.mjs
  var live_exports = {};
  __export(live_exports, {
    applyLiveMessage: () => applyLiveMessage,
    debounceWait: () => debounceWait,
    eventName: () => eventName,
    formPayload: () => formPayload,
    liveRoot: () => liveRoot,
    liveState: () => liveState,
    reconnectDelay: () => reconnectDelay,
    setLoading: () => setLoading,
    start: () => start4,
    wsURL: () => wsURL
  });
  function wsURL(loc, { view, topic } = {}) {
    if (!loc?.host) return "";
    const proto = loc.protocol === "https:" ? "wss:" : "ws:";
    const q = new URLSearchParams();
    if (view) q.set("view", view);
    if (topic) q.set("topic", topic);
    const qs = q.toString();
    return `${proto}//${loc.host}/amarra/live${qs ? `?${qs}` : ""}`;
  }
  function liveRoot(el) {
    return el?.closest?.("[amarra-live]") ?? null;
  }
  function eventName(el, kind) {
    if (!el?.getAttribute) return "";
    if (kind === "click") return el.getAttribute("amarra-click") || "";
    if (kind === "change") return el.getAttribute("amarra-change") || "";
    if (kind === "submit") return el.getAttribute("amarra-submit") || "";
    return "";
  }
  function formPayload(form, FormDataCtor = FormData) {
    if (!form || !FormDataCtor) return {};
    const data = {};
    for (const [k, v] of new FormDataCtor(form)) {
      data[k] = v;
    }
    return data;
  }
  function debounceWait(el) {
    const n = parseInt(el?.getAttribute?.("amarra-debounce") || "0", 10);
    return Number.isFinite(n) && n > 0 ? n : 0;
  }
  function setLoading(el, root, on) {
    const op = on ? "add" : "remove";
    el?.classList?.[op]?.("amarra-click-loading");
    root?.classList?.[op]?.("amarra-loading");
  }
  function applyLiveMessage(msg, root, morphFn = morph, extras = {}) {
    if (!msg || !root) return;
    if (msg.type !== "ok" && msg.type !== "morph" && msg.type !== "resync") return;
    const html = msg.html ?? "";
    if (html !== "") {
      if (msg.target) {
        const el = root.querySelector?.(`#${cssEscape(msg.target)}`) || root;
        morphFn(el, html, void 0, { morphStyle: "outerHTML" });
      } else {
        morphFn(root, html);
      }
    }
    const opFn = extras.applyOp ?? applyOp;
    const doc = extras.document ?? root.ownerDocument;
    for (const op of msg.ops || []) opFn(op, doc, extras);
    if (msg.patch) extras.history?.pushState?.({}, "", msg.patch);
    if (msg.navigate && extras.location) extras.location.href = msg.navigate;
    const pushFn = extras.dispatchPush ?? dispatchLivePush;
    for (const p of msg.pushes || []) pushFn(p.event, p.payload);
    if (msg.type === "resync" && doc && typeof doc.dispatchEvent === "function" && typeof CustomEvent === "function") {
      doc.dispatchEvent(
        new CustomEvent("amarra:live-resync", { bubbles: true, detail: { lost: msg.lost ?? 0 } })
      );
    }
    if (doc && typeof doc.dispatchEvent === "function") {
      doc.dispatchEvent(new CustomEvent("amarra:morphed", { bubbles: true }));
    }
  }
  function cssEscape(id) {
    if (typeof CSS !== "undefined" && CSS.escape) return CSS.escape(id);
    return String(id).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  }
  var LIVE_RECONNECT_CODES = /* @__PURE__ */ new Set([1003, 1008, 1011]);
  function reconnectDelay(attempt, opts = {}) {
    const base = opts.baseMs ?? 1e3;
    const max = opts.maxMs ?? 3e4;
    const factor = opts.factor ?? 2;
    const jitter = opts.jitter ?? 0.2;
    const random = opts.random ?? Math.random;
    const capped = Math.min(max, base * factor ** Math.max(0, attempt - 1));
    const spread = capped * jitter;
    return Math.max(0, Math.round(capped - spread + random() * spread * 2));
  }
  function liveState(root, state, detail = {}) {
    root?.setAttribute?.("data-amarra-live-state", state);
    if (typeof root?.dispatchEvent !== "function" || typeof CustomEvent !== "function") return;
    root.dispatchEvent(
      new CustomEvent("amarra:live-state", { bubbles: true, detail: { state, ...detail } })
    );
  }
  function start4(opts = {}) {
    const doc = opts.document ?? (typeof document !== "undefined" ? document : null);
    if (!doc || typeof doc.addEventListener !== "function") return;
    if (doc.documentElement?.dataset?.amarraLive === "true") return;
    if (doc.documentElement?.dataset) doc.documentElement.dataset.amarraLive = "true";
    const WS = opts.WebSocket ?? (typeof WebSocket !== "undefined" ? WebSocket : null);
    const location = opts.location ?? (typeof window !== "undefined" ? window.location : null);
    const connections = /* @__PURE__ */ new Map();
    const setTimer = opts.setTimeout ?? setTimeout;
    const clearTimer = opts.clearTimeout ?? clearTimeout;
    const disabled = opts.reconnectMs !== void 0 && opts.reconnectMs < 0;
    function removed(root) {
      return root.isConnected === false || typeof doc.contains === "function" && !doc.contains(root);
    }
    function onClose(root, record, ev) {
      record.ws = null;
      if (!connections.has(root)) return;
      if (removed(root)) {
        root?.removeAttribute?.("data-amarra-live-state");
        connections.delete(root);
        return;
      }
      const code = ev?.code ?? 1006;
      if (LIVE_RECONNECT_CODES.has(code)) {
        record.failed = true;
        liveState(root, "failed", {
          permanent: true,
          code,
          reason: ev?.reason ?? "",
          attempt: record.attempts
        });
        return;
      }
      if (disabled) {
        liveState(root, "closed", { code });
        return;
      }
      record.attempts += 1;
      const wait = reconnectDelay(record.attempts, {
        baseMs: opts.reconnectMs ?? 1e3,
        maxMs: opts.maxMs,
        jitter: opts.jitter,
        random: opts.random
      });
      liveState(root, "reconnecting", { code, attempt: record.attempts, retryInMs: wait });
      record.timer = setTimer(() => {
        record.timer = null;
        if (!connections.has(root) || removed(root)) {
          connections.delete(root);
          return;
        }
        open2(root, record);
      }, wait);
    }
    function open2(root, record) {
      const ws = new WS(record.url);
      record.ws = ws;
      liveState(root, "connecting", { attempt: record.attempts });
      ws.addEventListener("open", () => {
        record.attempts = 0;
        liveState(root, "open");
        const csrf = opts.csrfToken ?? csrfTokenFromMeta(doc);
        ws.send(JSON.stringify({ type: "join", csrf }));
      });
      ws.addEventListener("message", (ev) => {
        let msg;
        try {
          msg = JSON.parse(ev.data);
        } catch {
          return;
        }
        applyLiveMessage(msg, root, opts.morphFn, {
          document: doc,
          history: opts.history ?? (typeof window !== "undefined" ? window.history : null),
          location
        });
        setLoading(root._amarraPending, root, false);
        root._amarraPending = null;
      });
      ws.addEventListener("close", (ev) => onClose(root, record, ev));
    }
    function connect2(root) {
      if (!root || !WS || !location || connections.has(root)) return;
      const view = root.getAttribute("amarra-live");
      if (!view) return;
      const topic = root.getAttribute("data-amarra-topic") || view;
      const record = {
        url: wsURL(location, { view, topic }),
        ws: null,
        attempts: 0,
        timer: null,
        failed: false
      };
      connections.set(root, record);
      open2(root, record);
    }
    function sync() {
      for (const [root, record] of connections) {
        if (!removed(root)) continue;
        if (record.timer) clearTimer(record.timer);
        record.ws?.close?.();
        root?.removeAttribute?.("data-amarra-live-state");
        connections.delete(root);
      }
      doc.querySelectorAll?.("[amarra-live]").forEach((el) => {
        if (!connections.has(el)) connect2(el);
      });
    }
    sync();
    if (typeof doc.addEventListener === "function") doc.addEventListener("amarra:morphed", sync);
    function sendFrom(el, kind, extra) {
      const root = liveRoot(el);
      if (!root) return false;
      const name = eventName(kind === "submit" ? el : el.closest?.(`[amarra-${kind}]`) || el, kind);
      if (!name) return false;
      const ws = connections.get(root)?.ws;
      if (!ws || ws.readyState !== 1) return false;
      const payload = extra ?? {};
      const send = () => {
        setLoading(el, root, true);
        root._amarraPending = el;
        ws.send(JSON.stringify({ type: "event", event: name, payload, ref: String(Date.now()) }));
      };
      const wait = debounceWait(el);
      if (wait) {
        clearTimeout(el._amarraDebounce);
        el._amarraDebounce = setTimeout(send, wait);
        return true;
      }
      send();
      return true;
    }
    doc.addEventListener(
      "click",
      (ev) => {
        const btn = ev.target?.closest?.("[amarra-click]");
        if (!btn || !liveRoot(btn)) return;
        ev.preventDefault();
        sendFrom(btn, "click", { value: btn.value ?? btn.textContent ?? "" });
      },
      true
    );
    doc.addEventListener(
      "change",
      (ev) => {
        const el = ev.target?.closest?.("[amarra-change]");
        if (!el || !liveRoot(el)) return;
        sendFrom(el, "change", { value: el.value ?? "" });
      },
      true
    );
    doc.addEventListener(
      "submit",
      (ev) => {
        const form = ev.target?.closest?.("form[amarra-submit]");
        if (!form || !liveRoot(form)) return;
        ev.preventDefault();
        sendFrom(form, "submit", formPayload(form, opts.FormData));
      },
      true
    );
  }

  // pkg/amarra/js/entry.mjs
  if (typeof globalThis !== "undefined") {
    globalThis.Idiomorph = Idiomorph;
  }
  function boot() {
    if (typeof window === "undefined") return;
    start();
    start3();
    define();
    start2();
    start4();
    window.amarra = {
      drive: drive_exports,
      live: live_exports,
      hook: hook_exports
    };
  }
  if (typeof window !== "undefined") {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", boot);
    } else {
      boot();
    }
  }
})();
