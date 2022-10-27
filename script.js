  var $ = function(id) { return document.getElementById(id); }; //shorthand for document.getElementById command
  const form = $('form');
  
  var hideRequestTypeForms = function() {
    $('repair-form').style.display = 'none';
    $('assembly-form').style.display = 'none';
  }
  hideRequestTypeForms();

  $("request-type").addEventListener("change", function(e) {
    console.log($("request-type").value);
    if ($('request-type').value != 'empty') {
        $('continue-btn').disabled = false;
    }
    if ($('request-type').value == 'empty') {
        $('continue-btn').disabled = true;
    }

    // Show/Hide requestType forms based on selected request type value
    if ($('request-type').value == 'empty') {
      hideRequestTypeForms();
    }

    if ($('request-type').value == 'repair') {
      hideRequestTypeForms();
      $('repair-form').style.display = 'block';
    }

    if ($('request-type').value == 'assembly') {
      hideRequestTypeForms();
      $('assembly-form').style.display = 'block';
    }
  });

  form.addEventListener('submit', function(e) {
    e.preventDefault();

    var serializeForm = function (form) {
      var obj = {};
      var formData = new FormData(form);
      for (var key of formData.keys()) {
        obj[key] = formData.get(key);
      }
      return obj;
    };

    fetch('http://localhost:8080/"', { 
      method: 'POST',
      mode: 'cors',
      cache: 'no-cache',
      credentials: 'same-origin',
      redirect: 'follow',
      referrerPolicy: 'no-referrer',
      headers: {'Content-Type': 'application/json'}, 
      body: JSON.stringify(serializeForm(form))   //TODO: POST only non-empty form fields
    })
      .then(console.log(JSON.stringify(serializeForm(form)))) // FormData to JSON
      //.then(res => res.json())
      //.then(res => console.log(res)) // API response with received json
    })

