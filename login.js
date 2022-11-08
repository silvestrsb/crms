function Login() {
	let login = document.getElementById("login").value;
	let password = document.getElementById("password").value;
	
	loginReq(login, password)
}

async function loginReq(login, password) {
	let res = await fetch("http://localhost:8080/auth", {
		method: 'POST',
		mode: 'cors',
		cache: 'no-cache',
		credentials: 'same-origin',
		headers: {
		  'Content-Type': 'application/json'
		},
		redirect: 'follow',
		referrerPolicy: 'no-referrer',
		body: '{"login":"'+login+'","password":"'+password+'"}'
	  });
	let inf = await res;
	console.log(inf);
	
	let jso = await res.text();
	if (jso=="unknown") {
		document.getElementById("auth").innerHTML = "Incorrect username or password";
	} else {
		document.getElementById("auth").innerHTML = "";
		reloadWithSID(jso)
	}
}

async function reloadWithSID(sid) {
	let res = await fetch("http://localhost:8080/worker", {
		method: 'GET',
		mode: 'cors',
		cache: 'no-cache',
		credentials: 'same-origin',
		headers: {
		  'Content-Type': 'application/html',
		  'Session-ID': sid
		},
		redirect: 'follow',
		referrerPolicy: 'no-referrer',
	});
	let inf = await res;
	inf.text().then(function(text) {
		document.write(text);
	});
}

//example
async function getReq() {
	let res = await fetch("http://localhost:8080/data", {
		method: 'GET', // *GET, POST, PUT, DELETE, etc.
		mode: 'cors', // no-cors, *cors, same-origin
		cache: 'no-cache', // *default, no-cache, reload, force-cache, only-if-cached
		credentials: 'same-origin', // include, *same-origin, omit
		headers: {
		  'Content-Type': 'application/json',
		  'Session-ID': SID
		  // 'Content-Type': 'application/x-www-form-urlencoded',
		},
		redirect: 'follow', // manual, *follow, error
		referrerPolicy: 'no-referrer', // no-referrer, *client
	  });
	let inf = await res;
	console.log(inf);
	
	let jso = await res.text();
	if (jso == "success") {
		document.getElementById("resp").innerHTML = "You gained access to data";
	} else {
		document.getElementById("resp").innerHTML = "Unauthorized";
	}
}
