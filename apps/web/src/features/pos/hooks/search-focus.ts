export const POS_SEARCH_INPUT_ID = "pos-search";

/** Returns keyboard focus to the POS search input, where the barcode scanner types. */
export function focusSearch() {
  const input = document.getElementById(POS_SEARCH_INPUT_ID);
  if (input instanceof HTMLInputElement) {
    input.focus();
    input.select();
  }
}
