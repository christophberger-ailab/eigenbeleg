// Keep the selected entry of the list in view after it is re-rendered.
function revealSelected() {
	const selected = document.querySelector('#receipt-list [aria-current="page"]');
	if (selected) {
		selected.scrollIntoView({ block: "nearest" });
	}
}

document.addEventListener("DOMContentLoaded", revealSelected);
document.body.addEventListener("htmx:afterSettle", revealSelected);

// The server triggers a "download" event after saving a receipt with the
// PDF button. Navigating to the PDF URL downloads it without leaving the page.
document.body.addEventListener("download", (event) => {
	window.location.href = event.detail.value;
});
