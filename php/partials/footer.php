<?php // Layout footer ?>
</main>
</div>
<script>
// Pelabel tabel responsif: salin teks <th> ke atribut data-label tiap <td>
// sebaris, supaya CSS kartu-mobile bisa menampilkan label kolom tanpa
// mengubah satu pun template tabel. Dijalankan ulang setiap HTMX swap
// (daftar file dimuat parsial).
(function () {
  function labelTables(root) {
    (root || document).querySelectorAll('table').forEach(function (table) {
      var labels = [];
      table.querySelectorAll('thead th').forEach(function (th) {
        labels.push(th.textContent.trim());
      });
      table.querySelectorAll('tbody tr').forEach(function (tr) {
        var cells = tr.querySelectorAll('td');
        if (cells.length <= 1) { tr.classList.add('row-single'); return; }
        cells.forEach(function (td, i) {
          td.setAttribute('data-label', labels[i] || '');
        });
      });
    });
  }
  document.addEventListener('DOMContentLoaded', function () { labelTables(document); });
  labelTables(document);
  document.body.addEventListener('htmx:afterSwap', function (ev) { labelTables(ev.detail.target); });
})();
</script>
</body>
</html>
