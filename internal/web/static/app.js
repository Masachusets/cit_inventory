/**
 * CIT Inventory — модальные окна.
 *
 * Добавляет возможность перетаскивать модальные окна за заголовок.
 * Событие привязывается к document (делегирование), поэтому работает и для
 * окон, подставляемых htmx по запросу (например, при открытии формы карточки).
 */
(function () {
  'use strict';

  var interactiveSelector = 'button, input, textarea, select, a, [role="button"]';

  function isInteractiveTarget(target) {
    return target.closest(interactiveSelector) !== null;
  }

  function endDrag(content, doc) {
    if (!content || !doc) {
      return;
    }
    content.style.position = '';
    content.style.left = '';
    content.style.top = '';
    content.style.right = '';
    content.style.bottom = '';
    doc.removeEventListener('pointermove', onMove);
    doc.removeEventListener('pointerup', endDrag);
    doc.removeEventListener('pointercancel', endDrag);
  }

  var doc = document;

  doc.addEventListener('pointerdown', function (event) {
    var header = event.target.closest('.modal-header');
    if (!header || isInteractiveTarget(event.target)) {
      return;
    }

    var content = header.closest('.modal-content');
    if (!content) {
      return;
    }
    var modal = content.closest('.modal');
    if (!modal) {
      return;
    }

    event.preventDefault();

    var rect = content.getBoundingClientRect();
    var offsetX = event.clientX - rect.left;
    var offsetY = event.clientY - rect.top;

    function onMove(ev) {
      content.style.position = 'fixed';
      content.style.left = (ev.clientX - offsetX) + 'px';
      content.style.top = (ev.clientY - offsetY) + 'px';
      content.style.right = 'auto';
      content.style.bottom = 'auto';
    }

    doc.addEventListener('pointermove', onMove);
    doc.addEventListener('pointerup', endDrag);
    doc.addEventListener('pointercancel', endDrag);
  });
})();
