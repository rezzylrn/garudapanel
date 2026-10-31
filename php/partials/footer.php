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
  document.body.addEventListener('htmx:afterSwap', function (ev) { labelTables(ev.target); });

  // Anti double-submit: sekali form POST terkirim, tombol submit-nya
  // dikunci 5 detik supaya klik beruntun tidak mengirim request ganda
  // (create server dobel, upload dobel, dsb). Kunci dilepas otomatis
  // kalau halaman tidak pindah (mis. validasi gagal / request ajax).
  document.addEventListener('submit', function (ev) {
    var f = ev.target;
    if (!f || f.dataset.gpSubmitted) { ev.preventDefault(); return; }
    f.dataset.gpSubmitted = '1';
    var btns = f.querySelectorAll('button[type="submit"], input[type="submit"]');
    btns.forEach(function (b) { b.disabled = true; });
    setTimeout(function () {
      delete f.dataset.gpSubmitted;
      btns.forEach(function (b) { b.disabled = false; });
    }, 5000);
  });
})();
</script>
</body>
</html>
