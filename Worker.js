var $ = function(id) { return document.getElementById(id); }; //shorthand for document.getElementById command

var orderArray = [
    %s
]

buildTable(orderArray)

var j = 0;

function buildTable(data){
    var table = document.getElementById('ordersTable')

    for(i = 0; i < data.length; i++) {
        var row = `<tr>
                <td>${data[i].Piepr}</td>
                <td> 
                    <a data-bs-toggle="modal" data-bs-target="#modal-assembly" href="#" id=${i}>${data[i].Darb1}</a>
                </td>
                <td>${data[i].Darb2}</td>
                <td>${data[i].Darb3}</td>
            </tr>`    //change id=${i} -> id=${data[i].Id}
        j++;
        table.innerHTML += row    
    }
}

var hideRequestTypeForms = function() {
    $('complectation-form').style.display = 'none';
    $('purchace-form').style.display = 'none';
  }
  hideRequestTypeForms();


// Create event listener
document.addEventListener('click', (e) =>
{
// Retrieve id from clicked element
let elementId = e.target.id;
// If element has id
if (elementId !== '') {
    if (orderArray[parseInt(elementId)].Piepr.includes('Komplektēšana')) {
        hideRequestTypeForms();
        $('complectation-form').style.display = 'block';
    }
    else {
        hideRequestTypeForms();
        $('purchace-form').style.display = 'block';
    }
    console.log(elementId);
}
// If element has no id
else { 
    console.log("An element without an id was clicked.");
}
}
);
