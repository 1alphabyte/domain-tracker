if (localStorage.getItem("auth")) {
	location.assign("/dash/");
} else {
	const searchParms = new URLSearchParams(location.search);
	document.querySelector("form").addEventListener("submit", (e) => {
		e.preventDefault();
		fetch("/api/login", {
			method: "POST",
			body: JSON.stringify({ username: document.getElementById("username").value, password: document.getElementById("password").value })
		}).then(res => {
			if (res.ok) {
				localStorage.setItem("auth", true);
				if (searchParms.has("q") && searchParms.get("q").length > 0) 
					location.assign(`/dash/?q=${searchParms.get("q")}`);
				else
					location.assign("/dash/");
			} else if (res.status === 403 || res.status === 401) {
				document.getElementById("error-message").innerHTML = "Invalid username or password. <br /> Please try again.";
				document.getElementById("error-dialog").showModal();
			} else {
				document.getElementById("error-message").innerHTML = "Something went wrong. <br /> Please try again later.";
				document.getElementById("error-dialog").showModal();
			}
		});
	});
	console.info("Made with ❤️ by @1alphabyte https://github.com/1alphabyte");
}

document.getElementById("close-dialog").addEventListener("click", () => {
	document.getElementById("error-dialog").close();
});