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
			} else if (res.status === 403) {
				alert("Invalid username or password");
			} else {
				alert("An error occurred");
			}
		});
	});
	console.info("Made with ❤️ by @1alphabyte https://github.com/1alphabyte");
}