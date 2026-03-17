export function attachKeyboardBridge(controls, sendControl, onStatus) {
  const {
    buttonElement,
    inputElement,
    backspaceButton,
    enterButton,
  } = controls;
  let keyboardActive = false;
  let allowBlur = false;
  let previousValue = "";

  const syncUi = () => {
    buttonElement.classList.toggle("active", keyboardActive);
    inputElement.classList.toggle("active", keyboardActive);
  };

  const sendKeyPress = (key) => {
    sendControl({
      type: "input.keyDown",
      key,
    });
    sendControl({
      type: "input.keyUp",
      key,
    });
  };

  const sendText = (text) => {
    for (const char of text) {
      if (char === "\n") {
        sendKeyPress("Enter");
        continue;
      }

      sendControl({
        type: "input.text",
        text: char,
      });
    }
  };

  const focusInput = () => {
    inputElement.focus();
    const length = inputElement.value.length;
    inputElement.setSelectionRange(length, length);
  };

  const openKeyboard = () => {
    keyboardActive = true;
    previousValue = "";
    inputElement.value = "";
    syncUi();
    window.setTimeout(focusInput, 0);
    onStatus?.("Клавиатура активна");
  };

  const closeKeyboard = () => {
    keyboardActive = false;
    allowBlur = true;
    syncUi();
    inputElement.blur();
    inputElement.value = "";
    previousValue = "";
    onStatus?.("Клавиатура скрыта");
  };

  buttonElement.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    if (keyboardActive) {
      closeKeyboard();
      return;
    }
    openKeyboard();
  });

  const handleBackspace = (event) => {
    event.preventDefault();
    event.stopPropagation();
    sendKeyPress("Backspace");
    if (inputElement.value.length > 0) {
      inputElement.value = inputElement.value.slice(0, -1);
      previousValue = inputElement.value;
    }
    focusInput();
  };

  const handleEnter = (event) => {
    event.preventDefault();
    event.stopPropagation();
    sendKeyPress("Enter");
    inputElement.value = "";
    previousValue = "";
    focusInput();
  };

  const DEBOUNCE_MS = 200;
  const lastInvoke = { backspace: 0, enter: 0 };
  const wrapButtonHandler = (key, handler) => (event) => {
    if (event.type === "touchend") {
      event.preventDefault();
    }
    const now = Date.now();
    if (now - lastInvoke[key] < DEBOUNCE_MS) return;
    lastInvoke[key] = now;
    handler(event);
  };

  backspaceButton?.addEventListener("click", wrapButtonHandler("backspace", handleBackspace));
  backspaceButton?.addEventListener("touchend", wrapButtonHandler("backspace", handleBackspace), { passive: false });

  enterButton?.addEventListener("click", wrapButtonHandler("enter", handleEnter));
  enterButton?.addEventListener("touchend", wrapButtonHandler("enter", handleEnter), { passive: false });

  inputElement.addEventListener("input", (event) => {
    const value = event.target.value;
    if (!keyboardActive) {
      previousValue = value;
      return;
    }

    if (value.length > previousValue.length && value.startsWith(previousValue)) {
      sendText(value.slice(previousValue.length));
    } else if (value.length < previousValue.length) {
      const removed = previousValue.length - value.length;
      for (let index = 0; index < removed; index += 1) {
        sendKeyPress("Backspace");
      }
    } else if (value !== previousValue) {
      sendText(value);
    }

    previousValue = value;
  });

  inputElement.addEventListener("keydown", (event) => {
    if (event.key === "Backspace") return;
    sendControl({
      type: "input.keyDown",
      key: event.key,
      code: event.code,
      altKey: event.altKey,
      ctrlKey: event.ctrlKey,
      shiftKey: event.shiftKey,
      metaKey: event.metaKey,
    });
  });

  inputElement.addEventListener("keyup", (event) => {
    if (event.key === "Backspace") return;
    sendControl({
      type: "input.keyUp",
      key: event.key,
      code: event.code,
      altKey: event.altKey,
      ctrlKey: event.ctrlKey,
      shiftKey: event.shiftKey,
      metaKey: event.metaKey,
    });
    if (event.key === "Enter") {
      inputElement.value = "";
      previousValue = "";
    }
  });

  inputElement.addEventListener("blur", () => {
    if (!keyboardActive) {
      syncUi();
      return;
    }

    if (allowBlur) {
      allowBlur = false;
      syncUi();
      return;
    }

    window.setTimeout(() => {
      if (keyboardActive) {
        focusInput();
      }
    }, 0);
  });

  inputElement.addEventListener("touchstart", (event) => {
    event.stopPropagation();
  }, { passive: true });

  inputElement.addEventListener("click", (event) => {
    event.stopPropagation();
  });

  inputElement.addEventListener("focus", () => {
    if (!keyboardActive) {
      keyboardActive = true;
      syncUi();
    }
  });

  syncUi();

  return {
    isActive() {
      return keyboardActive;
    },
  };
}
