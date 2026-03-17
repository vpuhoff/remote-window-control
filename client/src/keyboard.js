export function attachKeyboardBridge(buttonElement, inputElement, sendControl) {
  buttonElement.addEventListener("click", () => {
    inputElement.focus();
  });

  inputElement.addEventListener("input", (event) => {
    const value = event.target.value;
    if (!value) {
      return;
    }

    for (const char of value) {
      sendControl({
        type: "input.text",
        text: char,
      });
    }

    event.target.value = "";
  });

  inputElement.addEventListener("keydown", (event) => {
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
    sendControl({
      type: "input.keyUp",
      key: event.key,
      code: event.code,
      altKey: event.altKey,
      ctrlKey: event.ctrlKey,
      shiftKey: event.shiftKey,
      metaKey: event.metaKey,
    });
  });
}
