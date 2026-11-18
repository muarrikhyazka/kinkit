/* Kinkit landing page: small, dependency-free enhancements.
 *  - points every "Start free" / "Sign in" link at the app (APP_URL below)
 *  - mobile menu, sticky-header shadow, scroll reveal, one-open-at-a-time FAQ
 *  - English / Bahasa Indonesia toggle (English is the default, matching the app UI)
 * The page works fully without JavaScript; this only adds polish.
 */
(function () {
  'use strict';

  // ---------------------------------------------------------------------
  // CONFIG: the only thing you normally need to change.
  // Base URL of the Kinkit web app. '' means "same origin as this page"
  // (e.g. landing served at / and the app on the same host).
  // Examples: 'https://app.kinkit.app'  or  'https://kinkit.example.com'
  // ---------------------------------------------------------------------
  var APP_URL = '';
  // A deploy can set it without editing this file: <meta name="kinkit-app-url" content="https://...">
  var appMeta = document.querySelector('meta[name="kinkit-app-url"]');
  if (appMeta && appMeta.content) APP_URL = appMeta.content;

  // Paths inside the app. The React app shows its sign-up form on /login
  // (toggle "Sign up"); point SIGNUP at '/login' if no /signup route exists.
  var APP_PATHS = { signup: '/login', login: '/login' };

  var root = document.documentElement;

  // ---------- app links ----------
  var base = APP_URL.replace(/\/+$/, '');
  document.querySelectorAll('[data-app-link]').forEach(function (a) {
    var path = APP_PATHS[a.getAttribute('data-app-link')];
    if (path) a.setAttribute('href', base + path);
  });

  // ---------- download sizes (HEAD request; hidden when the file is missing) ----------
  document.querySelectorAll('[data-dl-size]').forEach(function (el) {
    if (!window.fetch) return;
    fetch(el.getAttribute('data-dl-size'), { method: 'HEAD' }).then(function (r) {
      var n = r.ok ? Number(r.headers.get('content-length')) : 0;
      if (n > 0) el.textContent = n >= 1048576 ? (n / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB';
    }).catch(function () {});
  });

  // ---------- footer year ----------
  var year = document.getElementById('year');
  if (year) year.textContent = String(new Date().getFullYear());

  // ---------- mobile menu ----------
  var menuBtn = document.querySelector('.menu-btn');
  var navLinks = document.getElementById('nav-links');
  function setMenu(open) {
    if (!menuBtn || !navLinks) return;
    menuBtn.setAttribute('aria-expanded', String(open));
    navLinks.classList.toggle('open', open);
    menuBtn.setAttribute('aria-label', t(open ? 'nav.close' : 'nav.menu'));
  }
  if (menuBtn && navLinks) {
    menuBtn.addEventListener('click', function () {
      setMenu(menuBtn.getAttribute('aria-expanded') !== 'true');
    });
    navLinks.addEventListener('click', function (e) {
      if (e.target.closest('a')) setMenu(false);
    });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && menuBtn.getAttribute('aria-expanded') === 'true') {
        setMenu(false);
        menuBtn.focus();
      }
    });
    window.matchMedia('(min-width: 961px)').addEventListener('change', function (mq) {
      if (mq.matches) setMenu(false);
    });
  }

  // ---------- header shadow once scrolled ----------
  var header = document.querySelector('.site-header');
  function onScroll() {
    if (header) header.classList.toggle('scrolled', window.scrollY > 8);
  }
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();

  // ---------- scroll reveal ----------
  var reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var revealEls = document.querySelectorAll('.reveal');
  if (reduceMotion || !('IntersectionObserver' in window)) {
    revealEls.forEach(function (el) { el.classList.add('in'); });
  } else {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('in');
          io.unobserve(entry.target);
        }
      });
    }, { rootMargin: '0px 0px -8% 0px', threshold: 0.08 });
    revealEls.forEach(function (el) { io.observe(el); });
  }

  // ---------- FAQ: keep one answer open at a time ----------
  var faqItems = document.querySelectorAll('.faq details');
  faqItems.forEach(function (d) {
    d.addEventListener('toggle', function () {
      if (!d.open) return;
      faqItems.forEach(function (other) { if (other !== d) other.open = false; });
    });
  });

  // ---------- i18n ----------
  // English lives in the HTML. Indonesian strings below are keyed by the
  // element's data-i18n / data-i18n-aria attribute. Values may contain the
  // same inline markup as the English HTML (static, authored here).
  var ID = {
    'meta.title': 'Kinkit: keuangan, anggaran, dan tabungan keluarga dalam satu tempat',
    'skip': 'Langsung ke konten',
    'nav.features': 'Fitur',
    'nav.how': 'Cara kerja',
    'nav.screens': 'Semua layar',
    'nav.privacy': 'Data Anda',
    'nav.pricing': 'Harga',
    'nav.faq': 'Tanya jawab',
    'nav.download': 'Unduh',
    'dl.eyebrow': 'Unduh',
    'dl.title': 'Pasang Kinkit di HP dan TV',
    'dl.lead': 'Gratis dipasang. Masuk dengan akun yang sama seperti di web; aplikasi TV dipasangkan lewat tautan TV dan tidak pernah meminta kata sandi.',
    'dl.android.t': 'HP Android',
    'dl.android.d': 'Android 7 atau lebih baru. Buka filenya di HP dan izinkan "Instal aplikasi tidak dikenal" saat diminta.',
    'dl.atv.t': 'Android TV, Google TV &amp; Fire TV',
    'dl.atv.d': 'Pasang dengan aplikasi Downloader (ketik alamat halaman ini, lalu pilih filenya) atau dengan <code>adb install</code>. Pasangkan dengan tautan TV dari Pengaturan › Data &amp; perangkat.',
    'dl.webos.t': 'TV LG webOS',
    'dl.webos.d': 'Untuk TV dalam Developer Mode: pasang dengan <code>ares-install</code>. TV apa pun juga bisa langsung membuka tautan TV di browser.',
    'dl.tizen.t': 'TV Samsung Tizen',
    'dl.tizen.d': 'Samsung mewajibkan setiap paket ditandatangani dengan sertifikat Anda sendiri. Buka proyek ini di Tizen Studio dan jalankan di TV dalam Developer Mode, atau pakai tautan TV di browser.',
    'dl.web.t': 'iPhone, iPad &amp; komputer',
    'dl.web.d': 'Pakai aplikasi web di Safari atau browser apa pun, lalu pilih "Tambahkan ke Layar Utama" agar punya ikon sendiri. Aplikasi iPhone native segera hadir.',
    'dl.apk': 'Unduh APK',
    'dl.ipk': 'Unduh IPK',
    'dl.zip': 'Unduh proyek (.zip)',
    'dl.open': 'Buka aplikasi web',
    'nav.signin': 'Masuk',
    'nav.menu': 'Buka menu',
    'nav.close': 'Tutup menu',
    'cta.start': 'Mulai gratis',
    'cta.how': 'Lihat cara kerjanya',

    'hero.pillTag': 'Baru',
    'hero.pill': 'Aplikasi Android untuk HP dan TV sudah tersedia',
    'hero.title': 'Satu rumah untuk keuangan keluarga <span class="grad">dan semua anggotanya.</span>',
    'hero.lead': 'Kinkit membantu keluarga mencatat pengeluaran, menyusun anggaran, menabung untuk tujuan, dan menjaga semangat anak-anak, bersama-sama. Pakai di browser, di HP, di TV ruang keluarga, atau langsung dari Telegram dan Discord.',
    'hero.note1': 'Gratis untuk mulai',
    'hero.note2': 'Ekspor semua data, kapan saja',
    'hero.note3': 'Bisa di-host sendiri',
    'hero.visualAlt': 'Dasbor Kinkit di laptop dan HP, menampilkan pengeluaran bulan ini dibanding anggaran, pemasukan, tabungan, dan grafik pengeluaran per kategori.',
    'hero.float1': 'Lencana baru: Under Budget',
    'hero.float1s': 'Kebutuhan + Keinginan sesuai rencana',
    'hero.float2s': 'Dicatat dari Telegram',

    'trust.label': 'Kenapa keluarga bisa memercayai Kinkit',
    'trust.1': 'Bisa di-host sendiri dengan docker-compose',
    'trust.2': 'Ekspor data terbuka: CSV &amp; JSON',
    'trust.3': 'Bisa dipasang, jalan saat offline',
    'trust.4': 'Foto struk hanya untuk keluarga Anda',

    'features.eyebrow': 'Fitur',
    'features.title': 'Semua yang dibutuhkan keluarga, tanpa yang tidak perlu',
    'features.lead': 'Dari "25k makan siang" pertama sampai rekap akhir tahun, Kinkit mencakup keuangan, peran keluarga, motivasi, dan semua layar di rumah.',
    'g.money': 'Keuangan',
    'g.moneySub': 'Tahu ke mana uang pergi, dan rencanakan ke mana seharusnya.',
    'f.quick.t': 'Catat pengeluaran dalam hitungan detik',
    'f.quick.d': 'Tambah dari halaman mana saja. Ketik <kbd>25k</kbd>, <kbd>2rb</kbd>, atau <kbd>1.5jt</kbd>. Kinkit mengingat kategori untuk tiap deskripsi dan menampilkan entri terakhir sebagai pilihan sekali ketuk.',
    'f.budget.t': 'Anggaran yang menunjukkan kenyataan',
    'f.budget.d': 'Rencanakan pemasukan dan pengeluaran per kategori, dikelompokkan jadi Kebutuhan, Keinginan, dan Tabungan. Tiap sel menunjukkan yang benar-benar terpakai: hijau, kuning mulai 80%, merah jika lewat.',
    'f.alerts.t': 'Peringatan anggaran lewat chat',
    'f.alerts.d': 'Dapatkan pesan Telegram atau Discord saat sebuah kategori mencapai 80% anggaran bulanannya, dan sekali lagi jika terlampaui.',
    'f.income.t': 'Pemasukan: rencana vs diterima',
    'f.income.d': 'Catat uang yang benar-benar masuk dan bandingkan dengan rencana bulanan, tahunan, dan antartahun.',
    'f.recurring.t': 'Pembayaran rutin otomatis',
    'f.recurring.d': 'Masukkan sewa, langganan, atau cicilan sekali saja (bulanan, mingguan, atau tahunan), lalu Kinkit menambahkannya untuk Anda. Jeda dan lanjutkan kapan saja.',
    'f.goals.t': 'Target tabungan dan kekayaan bersih',
    'f.goals.d': 'Tiap target punya cincin progres dan menunjukkan berapa yang perlu ditabung per bulan agar tercapai tepat waktu. Pantau saldo akun dan progres menuju target kekayaan bersih.',
    'f.receipts.t': 'Struk yang terbaca sendiri',
    'f.receipts.d': 'Foto struk dan simpan bersama transaksinya. "Baca jumlah dari struk" menemukan totalnya langsung di perangkat Anda, jadi fotonya tidak dikirim ke mana pun untuk dibaca.',
    'f.csv.t': 'Impor CSV bank',
    'f.csv.d': 'Impor dari BCA, Mandiri, atau CSV umum. Pratinjau dan ubah kategori baris dulu. Mengimpor file yang sama dua kali tidak membuat duplikat, dan setiap impor bisa dibatalkan.',
    'f.dash.t': 'Dasbor sesuai selera',
    'f.dash.d': 'Tampilan Minggu, Bulan, Tahun, dan Antartahun, plus tab Kustom tempat Anda memilih grafik dan menyeret untuk mengurutkannya. Sembunyikan transaksi besar sekali jalan, lalu ekspor ke PDF.',
    'f.fx.t': 'Tampilan multi-mata uang',
    'f.fx.d': 'Lihat jumlah dalam IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, atau INR, dikonversi dengan kurs harian. Data tersimpan tetap dalam rupiah.',

    'g.home': 'Rumah &amp; keluarga',
    'g.homeSub': 'Satu keluarga, tiap orang dengan akses yang tepat.',
    'f.household.t': 'Satu keluarga, semua bisa masuk',
    'f.household.d': 'Tambahkan anggota keluarga dengan email dan kata sandi masing-masing. Semua orang mengelola keuangan keluarga yang sama.',
    'f.roles.t': 'Peran yang pas untuk keluarga nyata',
    'f.roles.d': 'Admin Keluarga dan Orang tua mengelola semuanya. Anak mencatat pengeluaran dan hanya bisa mengubah entrinya sendiri. Pengamat hanya bisa melihat, cocok untuk kakek-nenek atau akuntan.',
    'f.allowance.t': 'Uang saku anak',
    'f.allowance.d': 'Beri tiap anak uang saku bulanan dan biarkan mereka melihat berapa yang sudah terpakai bulan ini.',
    'f.undo.t': 'Urungkan, bukan "Yakin?"',
    'f.undo.d': 'Terhapus tanpa sengaja? Setiap penghapusan menampilkan tombol Urungkan selama beberapa detik, jadi tidak ada yang terjebak menjawab dialog konfirmasi.',
    'tag.soon': 'Segera hadir',
    'f.today.t': 'Hari ini',
    'f.today.d': 'Satu tampilan harian untuk semua yang jatuh tempo di rumah: pengingat, tugas, daftar, dan kondisi anggaran.',
    'f.chores.t': 'Tugas rumah, pengingat &amp; daftar belanja',
    'f.chores.d': 'Tugas rumah dengan poin, persetujuan orang tua, dan bukti foto yang dihitung ke uang saku anak. Pengingat rumah tangga untuk cucian, bersih-bersih, tandon air, tagihan, obat, dan ulang tahun, berulang setiap hari, tiap 2 minggu, atau tiap 3 bulan, yang mengingatkan orang yang tepat lewat Telegram atau Discord. Daftar belanja bersama: ketik "telur, 2 susu, roti x3", lalu jadikan belanjaan satu catatan pengeluaran.',

    'g.fun': 'Seru &amp; memotivasi',
    'g.funSub': 'Kebiasaan keuangan yang baik, jadi permainan keluarga.',
    'f.points.t': 'Poin dan level',
    'f.points.d': 'Dapatkan 10 poin per hari untuk mencatat, plus bonus streak. Naik dari Getting Started ke Tracker, Budget Buddy, Money Master, sampai Finance Wizard.',
    'f.badges.t': 'Streak dan lencana',
    'f.badges.d': 'Buka lencana First Log, Streak 7, Streak 30, First Budget Set, Under Budget, dan Saver, masing-masing dengan perayaan konfeti.',
    'f.challenges.t': 'Tantangan keluarga mingguan',
    'f.challenges.d': 'Catat setiap hari, akhir pekan tanpa belanja, jaga Keinginan di bawah jatah seminggu. Tiap tantangan menambah poin bonus ke total keluarga.',
    'f.leader.t': 'Papan peringkat bulanan',
    'f.leader.d': 'Lihat siapa yang paling banyak mengumpulkan poin bulan ini. Persaingan seru yang bikin semua mau mencatat.',
    'f.themes.t': 'Tema untuk segala usia',
    'f.themes.d': 'Terang, Gelap, tema Ceria untuk anak, dan tema Senior berkontras tinggi dengan teks lebih besar. <a href="#themes">Lihat temanya</a>.',

    'g.every': 'Di mana saja',
    'g.everySub': 'Web, HP, TV, dan aplikasi chat yang sudah Anda pakai.',
    'f.web.t': 'Aplikasi web yang bisa dipasang',
    'f.web.d': 'Berjalan di browser modern apa pun. Pasang di Android, iPhone, atau desktop untuk mendapat ikon sendiri dan jendela layar penuh.',
    'f.offline.t': 'Jalan saat offline',
    'f.offline.d': 'Tidak ada sinyal di pasar? Pengeluaran yang dicatat saat offline tersimpan di perangkat dan disinkronkan begitu Anda online lagi, dan tidak ada yang hilang diam-diam.',
    'f.tv.t': 'Tautan tampilan TV &amp; mobil',
    'f.tv.d': 'Buka tautan berbagi di browser TV, tablet dinding, atau head unit mobil untuk menampilkan angka bulan ini dengan huruf besar. Tidak perlu login, diperbarui tiap menit, dan bisa dicabut seketika.',
    'f.bots.t': 'Bot Telegram &amp; Discord',
    'f.bots.d': 'Kirim "50k makan siang" dan langsung tercatat, dengan kategori dipilih dari kata kunci Anda; kata baru muncul dengan tombol kategori dan bot mengingat pilihan Anda. Ketik <kbd>/recent</kbd> atau <kbd>/today</kbd>, buat pengingat dengan <kbd>/remind</kbd>, tambah daftar belanja dengan <kbd>/buy</kbd>, cek tugas dengan <kbd>/chores</kbd>, dan lihat rencana menu mingguan dengan <kbd>/menu</kbd>. Hubungkan bot Anda sendiri di Pengaturan.',
    'f.native.t': 'Aplikasi Android untuk HP',
    'f.native.d': 'Aplikasi native untuk seluruh keluarga: Hari ini, pengingat, tugas rumah, daftar belanja, pengeluaran, dan anggaran. <a href="#download">Unduh APK-nya</a>. Aplikasi iPhone segera hadir.',
    'f.smarttv.t': 'Penampil smart TV',
    'f.smarttv.d': 'Dasbor keluarga khusus tampilan untuk Android TV / Google TV / Fire TV dan LG webOS, plus paket Samsung Tizen untuk mode developer. <a href="#download">Dapatkan</a>.',

    'how.eyebrow': 'Cara kerja',
    'how.title': 'Siap dipakai dalam tiga langkah',
    'how.lead': 'Tanpa spreadsheet dan tanpa kata sandi bank. Cukup keluarga Anda, mencatat sambil jalan.',
    'how.s1.t': 'Buat rumah tangga Anda',
    'how.s1.d': 'Daftar dan Anda menjadi Admin Keluarga. Kinkit menyiapkan kategori dan sumber pemasukan awal yang masuk akal.',
    'how.s1.l1': 'Undang pasangan, anak, atau kakek-nenek',
    'how.s1.l2': 'Beri tiap orang peran dan uang saku',
    'how.s2.t': 'Catat sambil jalan',
    'how.s2.d': 'Pakai apa pun yang paling dekat. Semua entri masuk ke tempat yang sama.',
    'how.s2.l1': 'Tambah cepat di aplikasi, bahkan saat offline',
    'how.s2.l2': 'Pesan chat ke bot Telegram atau Discord Anda',
    'how.s2.l3': 'Foto struk, CSV bank, atau aturan pembayaran rutin',
    'how.s3.t': 'Lihat hasilnya, dan tetap di jalur',
    'how.s3.d': 'Anggaran, target, dan dasbor bercerita, dan permainannya menjaga semua tetap semangat.',
    'how.s3.l1': 'Anggaran vs realisasi, dengan peringatan di 80%',
    'how.s3.l2': 'Target dengan jumlah tabungan bulanan yang dibutuhkan',
    'how.s3.l3': 'Poin, lencana, dan tantangan mingguan',

    'screens.eyebrow': 'Semua layar',
    'screens.title': 'Di HP, laptop, dan TV ruang keluarga',
    'screens.lead': 'Keluarga yang sama dan angka yang sama, menyesuaikan di mana pun keluarga Anda berada.',
    'screens.phoneAlt': 'Kinkit di HP: pengeluaran bulan ini dibanding anggaran, pengeluaran terbaru, dan level poin.',
    'screens.webAlt': 'Kinkit di laptop: halaman Anggaran dengan jumlah rencana dan realisasi.',
    'screens.tvAlt': 'Tampilan TV Kinkit: pengeluaran bulan ini, sisa anggaran, dan pemasukan dengan huruf besar, diperbarui tiap menit.',
    'screens.phone.t': 'HP',
    'screens.phone.d': 'Pakai aplikasi Android, atau pasang aplikasi webnya di HP apa pun, lengkap dengan tab bawah, tombol + bulat, dan sinkron offline.',
    'screens.web.t': 'Web',
    'screens.web.d': 'Pengalaman lengkap: anggaran, impor, dasbor, pengaturan keluarga, dan ekspor PDF.',
    'screens.tv.t': 'TV &amp; mobil',
    'screens.tv.d': 'Buka tautan TV di browser TV atau head unit mobil mana pun, atau pasang aplikasi TV di Android TV / Google TV / Fire TV atau LG webOS.',
    'status.now': 'Tersedia sekarang',
    'status.android': 'Aplikasi Android: tersedia sekarang',
    'status.native': 'Aplikasi iPhone: segera hadir',
    'status.tvnow': 'Tautan TV: tersedia sekarang',
    'status.tvsoon': 'Aplikasi smart TV: tersedia sekarang',

    'themes.eyebrow': 'Tema untuk semua',
    'themes.title': 'Dari si kecil sampai kakek-nenek',
    'themes.lead': 'Tiap orang memilih tampilan yang cocok, di perangkatnya sendiri, lewat Pengaturan › Tampilan.',
    'themes.light.t': 'Terang',
    'themes.light.d': 'Bersih dan tenang untuk sehari-hari.',
    'themes.dark.t': 'Gelap',
    'themes.dark.d': 'Nyaman di mata saat malam.',
    'themes.playful.t': 'Ceria (anak)',
    'themes.playful.d': 'Warna hangat, sudut membulat, dan huruf yang ramah, dengan kontras yang tetap mudah dibaca.',
    'themes.senior.t': 'Senior (kontras tinggi)',
    'themes.senior.d': 'Teks lebih besar, tombol lebih besar, dan kontras hitam-putih yang tegas.',

    'privacy.eyebrow': 'Privasi &amp; kepemilikan data',
    'privacy.title': 'Data keluarga Anda tetap milik Anda',
    'privacy.lead': 'Kinkit tidak pernah meminta login bank Anda. Anda yang menentukan apa yang masuk, dan Anda bisa mengambil semuanya kapan saja.',
    'privacy.1.t': 'Ekspor kapan saja',
    'privacy.1.d': 'Unduh pengeluaran (per rentang tanggal) dan pemasukan sebagai CSV, atau cadangan lengkap semuanya sebagai JSON.',
    'privacy.2.t': 'Host sendiri',
    'privacy.2.d': 'Jalankan seluruh sistem (aplikasi, Postgres, dan bot chat) di server Anda sendiri dengan satu file docker-compose.',
    'privacy.3.t': 'Struk tetap privat',
    'privacy.3.d': 'Foto struk hanya bisa dilihat keluarga Anda, dan jumlahnya dibaca di perangkat Anda sendiri.',
    'privacy.4.t': 'Anda yang mengatur berbagi',
    'privacy.4.d': 'Tautan tampilan TV hanya menampilkan ringkasan bulan ini, dan bisa Anda cabut dengan sekali klik.',
    'privacy.termAlt': 'Terminal: menjalankan Kinkit dengan docker compose up -d, yang menyalakan Postgres, aplikasi di port 5010, serta bot Telegram dan Discord.',

    'price.eyebrow': 'Harga',
    'price.title': 'Paket sederhana untuk setiap keluarga',
    'price.lead': 'Mulai gratis. Tingkatkan saat keluarga butuh lebih, atau jalankan sendiri.',
    'price.perMonth': '/ bulan',
    'price.popular': 'Terbaik untuk keluarga',
    'price.free.t': 'Gratis',
    'price.free.d': 'Untuk mulai mengelola keuangan keluarga.',
    'price.free.1': 'Pengeluaran, pemasukan, anggaran &amp; target',
    'price.free.2': 'Dasbor &amp; ekspor PDF',
    'price.free.3': 'Ekspor CSV &amp; JSON',
    'price.free.4': 'Aplikasi web terpasang, mode offline',
    'price.family.t': 'Keluarga',
    'price.family.d': 'Untuk seisi rumah: anak, kakek-nenek, dan semua layar.',
    'price.family.1': 'Semua di paket Gratis',
    'price.family.2': 'Anggota keluarga, peran &amp; uang saku',
    'price.family.3': 'Poin, lencana &amp; tantangan mingguan',
    'price.family.4': 'Bot Telegram/Discord &amp; peringatan anggaran',
    'price.family.5': 'Struk, impor CSV bank, tautan TV',
    'price.family.cta': 'Mulai dengan Keluarga',
    'price.self.t': 'Host sendiri',
    'price.self.d': 'Untuk yang suka mengulik dan ingin semuanya di server sendiri.',
    'price.self.price': 'Gratis',
    'price.self.unit': '/ server Anda',
    'price.self.1': 'Aplikasi lengkap di perangkat Anda',
    'price.self.2': 'Satu file docker-compose',
    'price.self.3': 'Database Postgres dan cadangan milik Anda',
    'price.self.4': 'Dukungan komunitas',
    'price.self.cta': 'Cara host sendiri',
    'price.note': 'Paket dan harga masih pratinjau dan bisa berubah sebelum peluncuran.',

    'faq.eyebrow': 'Tanya jawab',
    'faq.title': 'Pertanyaan yang sering diajukan keluarga',
    'faq.q1': 'Apakah saya harus menghubungkan rekening bank?',
    'faq.a1': 'Tidak. Kinkit tidak pernah meminta kredensial bank. Catat pengeluaran di aplikasi atau lewat chat, atau impor CSV mutasi yang sudah bisa Anda unduh dari bank (BCA, Mandiri, atau format umum). Anda melihat pratinjau sebelum apa pun disimpan.',
    'faq.q2': 'Apakah anak-anak bisa memakainya?',
    'faq.a2': 'Bisa. Beri mereka peran Anak. Mereka bisa mencatat pengeluaran sendiri, hanya mengubah entri milik sendiri, dan melihat uang saku bulanannya. Mereka juga mendapat poin dan lencana, dan tema Ceria dibuat untuk mereka.',
    'faq.q3': 'Bisakah saya mengambil data saya?',
    'faq.a3': 'Selalu. Di Pengaturan › Data &amp; perangkat Anda bisa mengekspor pengeluaran (opsional per rentang tanggal) dan pemasukan sebagai CSV, atau mengunduh cadangan JSON lengkap.',
    'faq.q4': 'Bisakah saya meng-host Kinkit sendiri?',
    'faq.a4': 'Bisa. Repositorinya menyertakan file docker-compose yang menjalankan aplikasi Go, database Postgres, serta bot Telegram dan Discord. Isi beberapa rahasia di <code>.env</code> lalu jalankan <code>docker compose up -d</code>.',
    'faq.q5': 'Apakah bisa dipakai tanpa internet?',
    'faq.a5': 'Aplikasi yang terpasang tetap terbuka saat offline dan menampilkan data dari kunjungan terakhir. Pengeluaran yang dicatat saat offline disimpan di perangkat dan disinkronkan otomatis saat tersambung lagi. Jika server menolak sebuah entri, entri itu ditampilkan sehingga tidak ada yang hilang diam-diam.',
    'faq.q6': 'Bagaimana cara kerja bot Telegram dan Discord?',
    'faq.a6': 'Buat bot Anda sendiri (misalnya lewat BotFather di Telegram) lalu tempel token dan chat id-nya di Pengaturan › Bot. Setelah itu kirim pesan seperti "50k makan siang" untuk mencatat pengeluaran, atau <code>/recent</code> (<code>!recent</code> di Discord) untuk melihat entri terakhir. Bot yang sama juga mengirim peringatan anggaran.',
    'faq.q7': 'Mata uang apa saja yang didukung?',
    'faq.a7': 'Jumlah disimpan dalam rupiah (IDR). Anda bisa menampilkannya dalam IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, atau INR, dikonversi dengan kurs harian.',
    'faq.q8': 'Apakah ada aplikasi HP atau TV?',
    'faq.a8': 'Saat ini Anda bisa memasang aplikasi web di Android, iPhone, atau desktop, dan membuka tautan tampilan TV di browser TV atau mobil mana pun. Aplikasi native iOS/Android dan aplikasi smart TV khusus tampilan untuk Android TV / Google TV, Samsung Tizen, dan LG webOS segera hadir.',

    'final.title': 'Satukan keuangan keluarga Anda di rumah',
    'final.lead': 'Siapkan rumah tangga Anda dalam beberapa menit. Undang semua orang, catat pengeluaran pertama, dan raih lencana pertama hari ini.',

    'footer.top': 'Kinkit, kembali ke atas',
    'footer.about': 'Satu rumah untuk keuangan keluarga Anda. Dibuat untuk keluarga di Indonesia dan sekitarnya.',
    'footer.product': 'Produk',
    'footer.themes': 'Tema',
    'footer.data': 'Data Anda',
    'footer.export': 'Ekspor &amp; cadangan',
    'footer.selfhost': 'Host sendiri',
    'footer.account': 'Akun',
    'footer.back': 'Kembali ke atas ↑'
  };

  var LANG_KEY = 'kinkit_landing_lang';
  var EN = {}; // captured from the HTML on first switch
  var current = 'en';

  function t(key) {
    if (current === 'id' && ID[key]) return ID[key].replace(/&amp;/g, '&');
    if (key === 'nav.close') return 'Close menu';
    if (key === 'nav.menu') return 'Open menu';
    return EN[key] || key;
  }

  function captureEnglish() {
    if (EN.__done) return;
    document.querySelectorAll('[data-i18n]').forEach(function (el) {
      var k = el.getAttribute('data-i18n');
      if (!(k in EN)) EN[k] = el.innerHTML;
    });
    document.querySelectorAll('[data-i18n-aria]').forEach(function (el) {
      var k = el.getAttribute('data-i18n-aria');
      if (!(k in EN)) EN[k] = el.getAttribute('aria-label');
    });
    EN['meta.title'] = document.title;
    EN.__done = true;
  }

  function applyLang(lang) {
    captureEnglish();
    current = lang === 'id' ? 'id' : 'en';
    var dict = current === 'id' ? ID : EN;
    document.querySelectorAll('[data-i18n]').forEach(function (el) {
      var k = el.getAttribute('data-i18n');
      if (dict[k] != null) el.innerHTML = dict[k];
    });
    document.querySelectorAll('[data-i18n-aria]').forEach(function (el) {
      var k = el.getAttribute('data-i18n-aria');
      if (dict[k] != null) el.setAttribute('aria-label', dict[k].replace(/&amp;/g, '&'));
    });
    document.title = (dict['meta.title'] || EN['meta.title']);
    root.setAttribute('lang', current);
    document.querySelectorAll('.lang-toggle [data-lang]').forEach(function (b) {
      b.setAttribute('aria-pressed', String(b.getAttribute('data-lang') === current));
    });
    if (menuBtn) {
      menuBtn.setAttribute('aria-label', t(menuBtn.getAttribute('aria-expanded') === 'true' ? 'nav.close' : 'nav.menu'));
    }
  }

  document.querySelectorAll('.lang-toggle [data-lang]').forEach(function (b) {
    b.addEventListener('click', function () {
      var lang = b.getAttribute('data-lang');
      applyLang(lang);
      try { localStorage.setItem(LANG_KEY, lang); } catch (e) { /* storage blocked */ }
    });
  });

  // Saved choice wins; otherwise English (the app's UI language), or ?lang=id.
  var initial = null;
  try { initial = localStorage.getItem(LANG_KEY); } catch (e) { /* storage blocked */ }
  var param = /[?&]lang=(en|id)\b/.exec(window.location.search);
  if (param) initial = param[1];
  if (initial === 'id') applyLang('id');
})();
