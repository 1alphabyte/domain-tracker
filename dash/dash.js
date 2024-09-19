async function loadDomains() {
	const table = document.querySelector("table");
	const dropdown = document.getElementById("client");

	let domains = await fetch("/api/get")
		.then((res) => {
			if (res.ok) {
				return res.json();
			} else if (res.status == 401) {
				alert("Unauthorized");
				if (localStorage.getItem("auth")) localStorage.removeItem("auth");
				location.assign("/login/");
			} else {
				console.error(res);
				return null;
			}
		})
	let clients = await fetch("/api/clientList").then((res) => res.ok ? res.json() : null);

	if (!domains || !clients) {
		document.querySelector("body").innerHTML = "<h1>An error occured and Domain tracker is unable to proceed</h1><br /><h2>Please try again later</h2><br /><p>See the browser console for more information</p>";
		return;
	}
	clients.forEach((c) => {
		let option = document.createElement("option");
		option.value = c.id;
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

		domain.textContent = d.domain;
		domain.appendChild(edit);
		domain.appendChild(deleteBtn);
		exp.textContent = new Date(d.expiration).toLocaleDateString();
		ns.textContent = d.nameservers.split(",").join(", ");
		reg.textContent = d.registrar;
		client.textContent = clients.filter((c) => c.id == d.clientId)[0].name;
		raw.dataset.id = d.id;
		raw.textContent = "View";
		raw.className = "rawDataBtn";
		notes.dataset.id = d.id;
		notes.textContent = d.notes ? "View" : "None";
		d.notes ? notes.className = "rawDataBtn" : null;

		notes.addEventListener("click", (e) => {
			let id = e.target.dataset.id;
			let d = JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0].notes;
			let diag = document.getElementById("rawDataDiag");
			document.getElementById("rawDataDiagHeader").textContent = "Notes";
			let pre = document.getElementById("rawData");
			pre.textContent = d;
			diag.showModal	();
		});

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
			let clientId = document.getElementById("Eclient");
			let exp = document.getElementById("Expiration");
			let nameservers = document.getElementById("Nameservers");
			let currentDomain = JSON.parse(sessionStorage.getItem("domains")).filter((d) => d.id == id)[0];
			notes.value = currentDomain.notes;
			clientId.value = currentDomain.clientId;
			document.getElementById("Edomain").textContent = currentDomain.domain;
			exp.value = new Date(currentDomain.expiration).toISOString().split("T")[0];
			nameservers.value = currentDomain.nameservers;
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
		row.appendChild(reg);
		row.appendChild(client);
		row.appendChild(notes);
		row.appendChild(raw);

		table.appendChild(row);
	});

}

function main() {
	if (window.matchMedia("(max-width: 850px)").matches) {
		document.querySelector("body").innerHTML = "<h1>Screen too small</h1>";
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
	loadDomains();
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
			body: JSON.stringify({ domain, clientId, notes }),
		}).then((res) => {
			if (res.ok) {
				alert("Domain added");
				location.reload();
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
				id: e.target.dataset.id,
				clientId: document.getElementById("Eclient").value,
				expiration: document.getElementById("Expiration").value,
				nameservers: document.getElementById("Nameservers").value,
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
	document.getElementById("delCBtn").addEventListener("click", () => {
		let id = document.getElementById("delCSel").value;
		if (id === "null") {
			alert("Please select a client");
			return;
		}
		if (confirm("Are you sure?\nThis action cannot be undone.\nAll domains for this client MUST be deleted otherwise this will FAIL!")) {
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