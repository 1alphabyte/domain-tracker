const table = document.querySelector("table");
const searchParms = new URLSearchParams(location.search);

async function loadDomains() {
	const dropdown = document.getElementById("client");

	let domains = await fetch("/api/get")
		.then((res) => {
			if (res.status === 200) {
				return res.json();
			} else if (res.status == 401) {
				alert("Unauthorized");
				if (localStorage.getItem("auth")) localStorage.removeItem("auth");
				if (searchParms.has("q") && searchParms.get("q").length > 0)
					location.assign(`/login/?q=${searchParms.get("q")}`);
				else
					location.assign("/login/");
			} else if (res.status == 204) {
				return [];
			} else {
				console.error(res);
				return null;
			}
		})
	let clients = await fetch("/api/clientList").then((res) => {
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
		document.getElementById("Eclient").appendChild(option.cloneNode(true));
	});
	let newDropdown = dropdown.cloneNode(true);
	newDropdown.id = "delCSel";
	document.getElementById("delCName").appendChild(newDropdown);
	sessionStorage.setItem("domains", JSON.stringify(domains));
	domains.forEach((d) => {
		let row = document.createElement("tr");
		let domain = document.createElement("td");
		let exp = document.createElement("td");
		let ns = document.createElement("td");
		let aDNS = document.createElement("td");
		let aaaaDNS = document.createElement("td");
		let mxDNS = document.createElement("td");
		let reg = document.createElement("td");
		let client = document.createElement("td");
		let notes = document.createElement("td");
		let raw = document.createElement("td");
		let deleteBtn = document.createElement("span");
		let edit = document.createElement("span");
		edit.className = "editIcon";
		edit.title = "Edit";
		edit.dataset.id = d.id;
		deleteBtn.className = "deleteIcon";
		deleteBtn.title = "Delete";
		deleteBtn.dataset.id = d.id;
		aDNS.className = "dnsRecord";
		aaaaDNS.className = "dnsRecord";
		mxDNS.className = "dnsRecord";
		aDNS.hidden = true;
		aaaaDNS.hidden = true;
		mxDNS.hidden = true;
		ns.className = "nameServer";

		domain.textContent = d.domain;
		domain.appendChild(edit);
		domain.appendChild(deleteBtn);
		exp.textContent = new Date(d.expiration).toLocaleDateString();
		ns.textContent = d.nameservers ? d.nameservers.join(", ") : "None ❌";
		let dnsD = d.dns;
		dnsD.a ? aDNS.textContent = dnsD.a : aDNS.textContent = "None ❌";
		dnsD.aaaa ? aaaaDNS.textContent = dnsD.aaaa : aaaaDNS.textContent = "None ❌";
		let mxClickable = false;
		if (dnsD.mx.split(",").length > 1) {
			mxDNS.textContent = "View";
			mxClickable = true;
		} else if (dnsD.mx) {
			mxDNS.textContent = dnsD.mx;
		} else {
			mxDNS.textContent = "None ❌";
		}
		reg.textContent = d.registrar;
		client.textContent = clients.filter((c) => c.id == d.clientId)[0].name;
		raw.dataset.id = d.id;
		raw.textContent = "View";
		raw.className = "rawDataBtn";
		notes.textContent = d.notes ? "View" : "None ❌";

		if (mxClickable) {
			mxDNS.classList.add("rawDataBtn");
			mxDNS.dataset.id = d.id;
			mxDNS.addEventListener("click", (e) => {
				let id = e.target.dataset.id;
				let d = JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0].dns;
				let diag = document.getElementById("rawDataDiag");
				document.getElementById("rawDataDiagHeader").textContent = "DNS: MX records";
				document.getElementById("rawData").textContent = d.mx.split(",").sort((a, b) => a.split(" ")[0] - b.split(" ")[0]).join("\n");
				diag.showModal();
			});
		}

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
			let d = JSON.parse(JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0].rawWhoisData);
			let diag = document.getElementById("rawDataDiag");
			let pre = document.getElementById("rawData");
			document.getElementById("rawDataDiagHeader").textContent = "Raw Data";
			pre.textContent = JSON.stringify(d, null, 1);
			diag.showModal();
		});

		edit.addEventListener("click", (e) => {
			let id = e.target.dataset.id;
			document.getElementById('editDDiag').showModal();
			let notes = document.getElementById("Enotes");
			let clientID = document.getElementById("Eclient");
			let currentDomain = JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0];
			notes.value = currentDomain.notes;
			clientID.value = currentDomain.clientID;
			document.getElementById("Edomain").textContent = currentDomain.domain;
			document.getElementById("editDForm").dataset.id = id;
		});

		deleteBtn.addEventListener("click", (e) => {
			let id = e.target.dataset.id;
			if (confirm("Are you sure you want to delete this domain?\nTHIS ACTION CANNOT BE UNDONE")) {
				fetch(`/api/delete/${id}`, {
					method: "DELETE",
				}).then((res) => {
					if (res.ok) {
						alert("Domain deleted");
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
		row.appendChild(ns);
		row.appendChild(aaaaDNS);
		row.appendChild(mxDNS);
		row.appendChild(aDNS);
		row.appendChild(reg);
		row.appendChild(client);
		row.appendChild(notes);
		row.appendChild(raw);

		table.appendChild(row);
	});

}

function main() {
	if (window.matchMedia("(max-width: 850px)").matches) {
		document.querySelector("body").innerHTML = `<h1>Your screen size doesn't meet the minimum requirements</h1><br /><p>Current width: ${window.innerWidth}px</p><p>Minimum width: 850px</p><p>Please resize your window or use a different device</p>`;
		window.addEventListener("resize", () => {
			document.querySelector("body").innerHTML = `<h1>Your screen size doesn't meet the minimum requirements</h1><br /><p>Current width: ${window.innerWidth}px</p><p>Minimum width: 850px</p><p>Please resize your window or use a different device</p>`;
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
		fetch("/api/add", {
			method: "POST",
			body: JSON.stringify({ domain, clientID: parseInt(clientId), notes }),
		}).then((res) => {
			if (res.ok) {
				alert("Domain added");
				location.reload();
			} else if (res.status === 409) {
				alert("Domain already exists\nDomains can only be added once");
				location.assign(`./?q=${domain}`);
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

	document.getElementById("editDForm").addEventListener("submit", (e) => {
		e.preventDefault();
		fetch("/api/edit", {
			method: "POST",
			body: JSON.stringify({
				id: parseInt(e.target.dataset.id),
				clientId: parseInt(document.getElementById("Eclient").value),
				notes: document.getElementById("Enotes").value,
			}),
		}).then((res) => {
			if (res.ok) {
				alert("Domain updated");
				location.reload();
			} else {
				alert("Error updating domain");
			}
		});
	});

	document.getElementById("delC").addEventListener("click", () => {
		document.getElementById("delCDiag").showModal();
	});
	search.addEventListener("input", (e) => {
		const rows = Array.from(table.rows).slice(1); // Exclude header row
		const searchTerm = e.target.value.toLowerCase();

		rows.forEach((row) => {
			const cellText = row.cells[0].firstChild.textContent.toLowerCase();
			row.hidden = !cellText.includes(searchTerm);
		});
	});
	document.getElementById("delCBtn").addEventListener("click", () => {
		let id = document.getElementById("delCSel").value;
		if (id === "null") {
			alert("Please select a client");
			return;
		}
		if (confirm("Are you sure?\nThis action cannot be undone.\nAll domains for this client MUST be deleted (including in TLS tracker) otherwise this will FAIL!")) {
			fetch(`/api/deleteClient/${id}`, {
				method: "DELETE",
			}).then(async (res) => {
				if (res.ok) {
					alert("Client deleted");
					location.reload();
				} else if (res.status === 409) {
					alert(await res.text());
				} else {
					alert("Error deleting client");
				}
			});
		}
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

document.getElementById("tableHeader2").addEventListener("click", () => {
	document.getElementById("ns").hidden = true;
	document.getElementById("a").hidden = false;
	document.getElementById("aaaa").hidden = false;
	document.getElementById("mx").hidden = false;
	Array.from(document.getElementsByClassName("nameServer")).forEach((e) => e.hidden = true);
	Array.from(document.getElementsByClassName("dnsRecord")).forEach((e) => e.hidden = false);
});

document.getElementById("tableHeader22").addEventListener("click", () => {
	document.getElementById("ns").hidden = false;
	document.getElementById("a").hidden = true;
	document.getElementById("aaaa").hidden = true;
	document.getElementById("mx").hidden = true;
	Array.from(document.getElementsByClassName("nameServer")).forEach((e) => e.hidden = false);
	Array.from(document.getElementsByClassName("dnsRecord")).forEach((e) => e.hidden = true);
});