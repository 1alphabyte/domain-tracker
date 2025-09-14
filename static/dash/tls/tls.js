const table = document.querySelector("table");
const searchParms = new URLSearchParams(location.search);

async function loadDomains() {
	const dropdown = document.getElementById("client");

	let domains = await fetch("/api/tlsList")
		.then((res) => {
			if (res.status == 200) {
				return res.json();
			} else if (res.status == 204) {
				return [];
			} else if (res.status == 401) {
				alert("Unauthorized");
				if (localStorage.getItem("auth")) localStorage.removeItem("auth");
				if (searchParms.has("q") && searchParms.get("q").length > 0)
					location.assign(`/login/?q=${searchParms.get("q")}&backTo=tls`);
				location.assign("/login/?backTo=tls");
			} else {
				console.error(res);
				return null;
			}
		})
		let clients = await fetch("/api/clientList")
			.then((res) => {
				if (res.status === 200) {
					return res.json();
				} else if (res.status == 204) {
					return [];
				} else {
					console.error(res);
					return null;
				}
			});
	if (!domains || !clients) {
		document.querySelector("body").innerHTML = "<h1>An error occurred and Domain tracker is unable to proceed</h1><br /><h2>Please try again later</h2><br /><p>See the browser console for more information</p>";
		return;
	}
	clients.forEach((c) => {
		let option = document.createElement("option");
		option.value = c.ID;
		option.textContent = c.name;
		dropdown.appendChild(option);
	});
	sessionStorage.setItem("domains", JSON.stringify(domains));
	domains.forEach((d) => {
		let row = document.createElement("tr");
		let domain = document.createElement("td");
		let exp = document.createElement("td");
		let auth = document.createElement("td");
		let client = document.createElement("td");
		let notes = document.createElement("td");
		let raw = document.createElement("td");
		let deleteBtn = document.createElement("span");
		deleteBtn.className = "deleteIcon";
		deleteBtn.title = "Delete";
		deleteBtn.dataset.id = d.id;

		domain.textContent = d.commonName;
		domain.appendChild(deleteBtn);
		exp.textContent = new Date(d.expiration).toLocaleDateString();
		auth.textContent = d.authority;
		client.textContent = clients.filter((c) => c.id == d.clientId)[0].name;
		raw.dataset.id = d.id;
		raw.textContent = "View";
		raw.className = "rawDataBtn";
		notes.textContent = d.notes ? "View" : "None";

		if (d.notes) {
			notes.className = "rawDataBtn";
			notes.dataset.id = d.id;
			notes.addEventListener("click", (e) => {
				let id = e.target.dataset.id;
				let d = JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0].notes;
				let diag = document.getElementById("rawDataDiag");
				document.getElementById("rawDataDiagHeader").textContent = "Notes";
				let pre = document.getElementById("rawData");
				pre.textContent = d;
				diag.showModal();
			});
		}

		raw.addEventListener("click", (e) => {
			let id = e.target.dataset.id;
			let d = JSON.parse(JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0].rawData);
			let diag = document.getElementById("rawDataDiag");
			let pre = document.getElementById("rawData");
			document.getElementById("rawDataDiagHeader").textContent = "Raw Data";
			pre.textContent = JSON.stringify(d, null, 1);
			diag.showModal();
		});

		deleteBtn.addEventListener("click", (e) => {
			let id = e.target.dataset.id;
			if (confirm("Are you sure you want to delete this domain?\nTHIS ACTION CANNOT BE UNDONE")) {
				fetch(`/api/tlsDelete/${id}`, {
					method: "DELETE",
				}).then((res) => {
					if (res.ok) {
						alert("Website deleted");
						location.reload();
					} else {
						alert("Error deleting domain");
					}
				});
			} else {
				return alert("Action aborted");
			}
		});

		row.appendChild(domain);
		row.appendChild(exp);
		row.appendChild(auth);
		row.appendChild(client);
		row.appendChild(notes);
		row.appendChild(raw);

		table.appendChild(row);
	});

}

function main() {
	if (window.matchMedia("(max-width: 850px)").matches) {
		document.querySelector("body").innerHTML = "<h1>Your screen size doesn't meet the minimum requirements</h1>";
		window.addEventListener("resize", () => {
			if (window.matchMedia("(min-width: 850px)").matches) {
				location.reload();
			}
		});
		return;
	}
	window.addEventListener("resize", () => {
		if (window.matchMedia("(max-width: 850px)").matches) {
			location.reload();
		}
	});
	let search = document.getElementById("searchInput");
	loadDomains().then(() => {
		if (searchParms.has("q") && searchParms.get("q").length > 0) {
			search.value = searchParms.get("q");
			search.dispatchEvent(new Event('input'));
		}
		document.getElementById("tableHeader1").dispatchEvent(new Event("click"));
	});
	document.getElementById("addDForm").addEventListener("submit", (e) => {
		e.preventDefault();
		let clientId = document.getElementById("client").value;
		if (clientId === "null") {
			alert("Please select a client");
			return;
		}
		document.getElementById('addDDiag').close();
		let domain = document.getElementById("domain").value;
		let notes = document.getElementById("notes").value;
		document.getElementById("addDForm").reset();
		fetch("/api/tlsAddDomain", {
			method: "POST",
			body: JSON.stringify({ domain, clientId: parseInt(clientId), notes }),
		}).then(async (res) => {
			if (res.ok) {
				alert("Domain added");
				location.reload();
			} else if (res.status === 409) {
				alert("Domain already exists\nThis domain may share a certificate common name with another domain");
				location.assign(`./?q=${await res.text()}`);
				
			} else {
				alert("Error adding domain");
			}
		});

	});

	document.getElementById("addCForm").addEventListener("submit", (e) => {
		e.preventDefault();
		document.getElementById('AddCDiag').close();
		let name = document.getElementById("clientName").value;
		document.getElementById("addCForm").reset();
		fetch("/api/clientAdd", {
			method: "POST",
			body: JSON.stringify({ name }),
		}).then((res) => {
			if (res.ok) {
				alert("Client added");
				location.reload();
			} else {
				alert("Error adding client");
			}
		});
	});


	document.getElementById("addD").addEventListener("click", () => {
		document.getElementById("addDDiag").showModal();
	});
	document.getElementById("AddC").addEventListener("click", () => {
		document.getElementById("AddCDiag").showModal();
	});

	document.querySelectorAll(".closeDiag").forEach((b) => {
		b.addEventListener("click", (e) => {
			e.target.parentElement.close();
		});
	});

	search.addEventListener("input", (e) => {
		const rows = Array.from(table.rows).slice(1); // Exclude header row
		const searchTerm = e.target.value.toLowerCase();

		rows.forEach((row) => {
			const cellText = row.cells[0].firstChild.textContent.toLowerCase();
			row.hidden = !cellText.includes(searchTerm);
		});
	});
}

main();


function sortTable(columnIndex) {
	const rows = Array.from(table.rows).slice(1); // Exclude header row
	let direction = table.getAttribute('data-sort-direction') === 'asc' ? 'desc' : 'asc';
	table.setAttribute('data-sort-direction', direction);
	let tableHeader = document.getElementById(`tableHeader${columnIndex}`);
	if (direction === 'asc') {
		tableHeader.classList = "arrow up";
	} else {
		tableHeader.classList = "arrow down";
	}
	// Reset other headers
	document.querySelectorAll(".arrow").forEach((e) => {
		if (e !== tableHeader) {
			e.classList = "arrow left";
		}
	});

	rows.sort((a, b) => {
		const x = a.cells[columnIndex].innerText.toLowerCase();
		const y = b.cells[columnIndex].innerText.toLowerCase();
		if (direction === 'asc') {
			return x > y ? 1 : x < y ? -1 : 0;
		} else {
			return x < y ? 1 : x > y ? -1 : 0;
		}
	});

	rows.forEach(row => table.appendChild(row));
}

function sortTableDate(columnIndex) {
	const rows = Array.from(table.rows).slice(1); // Exclude header row
	let direction = table.getAttribute('data-sort-direction') === 'asc' ? 'desc' : 'asc';
	table.setAttribute('data-sort-direction', direction);
	let tableHeader = document.getElementById(`tableHeader${columnIndex}`);
	if (direction === 'asc') {
		tableHeader.classList = "arrow up";
	} else {
		tableHeader.classList = "arrow down";
	}
	// Reset other headers
	document.querySelectorAll(".arrow").forEach((e) => {
		if (e !== tableHeader) {
			e.classList = "arrow left";
		}
	});

	rows.sort((a, b) => {
		const x = new Date(a.cells[columnIndex].innerText);
		const y = new Date(b.cells[columnIndex].innerText);
		if (direction === 'asc') {
			return x - y;
		} else {
			return y - x;
		}
	});

	rows.forEach(row => table.appendChild(row));
}