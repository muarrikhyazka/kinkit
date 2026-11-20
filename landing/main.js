/* Kinkit landing page: small, dependency-free enhancements.
 *  - points every "Start free" / "Sign in" link at the app (APP_URL below)
 *  - mobile menu, sticky-header shadow, scroll reveal, Kiki in-view motion, one-open-at-a-time FAQ
 *  - English / Bahasa Indonesia toggle (English is the default, matching the app UI)
 * The page works fully without JavaScript; this only adds polish.
 */
(function () {
  'use strict';

  // CONFIG: the only thing you normally need to change.
  // Base URL of the Kinkit web app. '' means "same origin as this page"
  // (e.g. landing served at / and the app on the same host).
  // Examples: 'https://app.kinkit.app'  or  'https://kinkit.example.com'
  var APP_URL = '';
  // A deploy can set it without editing this file: <meta name="kinkit-app-url" content="https://...">
  var appMeta = document.querySelector('meta[name="kinkit-app-url"]');
  if (appMeta && appMeta.content) APP_URL = appMeta.content;

  // Paths inside the app. The React app shows its sign-up form on /login
  // (toggle "Sign up"); point SIGNUP at '/login' if no /signup route exists.
  var APP_PATHS = { signup: '/login', login: '/login', docs: '/docs' }; // docs: the in-app documentation site; with no app URL set it stays a relative /docs

  var root = document.documentElement;

  // app links
  var base = APP_URL.replace(/\/+$/, '');
  document.querySelectorAll('[data-app-link]').forEach(function (a) {
    var path = APP_PATHS[a.getAttribute('data-app-link')];
    if (path) a.setAttribute('href', base + path);
  });

  // download sizes (HEAD request; hidden when the file is missing)
  document.querySelectorAll('[data-dl-size]').forEach(function (el) {
    if (!window.fetch) return;
    fetch(el.getAttribute('data-dl-size'), { method: 'HEAD' }).then(function (r) {
      var n = r.ok ? Number(r.headers.get('content-length')) : 0;
      if (n > 0) el.textContent = n >= 1048576 ? (n / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB';
    }).catch(function () {});
  });

  // footer year
  var year = document.getElementById('year');
  if (year) year.textContent = String(new Date().getFullYear());

  // mobile menu
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

  // header shadow once scrolled
  var header = document.querySelector('.site-header');
  function onScroll() {
    if (header) header.classList.toggle('scrolled', window.scrollY > 8);
  }
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();

  // scroll reveal
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

  // Kiki: start the celebrate / relax motion when it scrolls into view
  // (CSS only animates under prefers-reduced-motion: no-preference, so this is safe either way)
  var mascots = document.querySelectorAll('.mk[data-mk-animate]');
  if (!reduceMotion && 'IntersectionObserver' in window) {
    var mio = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('mk--animate');
          mio.unobserve(entry.target);
        }
      });
    }, { threshold: 0.4 });
    mascots.forEach(function (el) { mio.observe(el); });
  }

  // FAQ: keep one answer open at a time
  var faqItems = document.querySelectorAll('.faq details');
  faqItems.forEach(function (d) {
    d.addEventListener('toggle', function () {
      if (!d.open) return;
      faqItems.forEach(function (other) { if (other !== d) other.open = false; });
    });
  });

  // i18n
  // English lives in the HTML. Indonesian strings below are keyed by the
  // element's data-i18n / data-i18n-aria attribute. Values may contain the
  // same inline markup as the English HTML (static, authored here).
  var ID = {
    'meta.title': 'Kinkit: kelola seluruh rumah tangga, bukan hanya anggarannya',
    'skip': 'Langsung ke konten',
    'nav.features': 'Fitur',
    'nav.how': 'Lihat cara kerjanya',
    'nav.privacy': 'Data Anda',
    'nav.faq': 'Tanya jawab',
    'nav.docs': 'Dokumentasi',
    'nav.download': 'Unduh',
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

    'hero.title': 'Kelola seluruh rumah tangga, <span class="soft">bukan hanya anggarannya.</span>',
    'hero.lead': 'Uang, tugas rumah, pengingat, kalender keluarga, dan menu mingguan, dipakai bersama seisi rumah. Catat belanja cukup dengan mengirim pesan ke bot Telegram, Discord, Slack, atau Matrix, dan biarkan anak mengumpulkan poin karena ikut membantu.',
    'hero.note': 'Gratis untuk mendaftar. Kinkit tidak pernah meminta login bank Anda.',
    'hero.visualAlt': 'Dasbor Kinkit di laptop, menampilkan pengeluaran bulan ini dibanding anggaran, pemasukan, tabungan, dan pengeluaran per kategori, dengan maskot Kiki yang melambai.',

    'trust.label': 'Kenapa keluarga bisa memercayai Kinkit',
    'trust.1': 'Tidak pernah meminta login bank Anda',
    'trust.2': 'Ekspor data terbuka: CSV &amp; JSON',
    'trust.3': 'Bisa dipasang, jalan saat offline',
    'trust.4': 'Foto struk hanya untuk keluarga Anda',

    'features.title': 'Keuangan, rumah, dan keluarga dalam satu aplikasi',
    'features.lead': 'Dari "25k makan siang" pertama sampai rekap akhir tahun. Tiga bagian, satu rumah tangga.',

    'g.money': 'Keuangan',
    'g.moneySub': 'Tahu ke mana uang pergi, dan rencanakan ke mana seharusnya.',

    'f.quick.t': 'Catat pengeluaran dalam hitungan detik',
    'f.quick.d': 'Ketik <kbd>25k</kbd>, <kbd>2rb</kbd>, atau <kbd>1.5jt</kbd> dari halaman mana saja. Kinkit mengingat kategori untuk tiap deskripsi.',
    'f.budget.t': 'Anggaran yang menunjukkan kenyataan',
    'f.budget.d': 'Rencanakan per kategori sebagai Kebutuhan, Keinginan, dan Tabungan. Tiap sel jadi kuning di 80% dan merah saat terlampaui.',
    'f.alerts.t': 'Peringatan anggaran lewat chat',
    'f.alerts.d': 'Pesan di Telegram, Discord, Slack, atau Matrix saat sebuah kategori mencapai 80%, dan sekali lagi jika terlampaui.',
    'f.income.t': 'Pemasukan: rencana vs diterima',
    'f.income.d': 'Bandingkan uang yang masuk dengan rencana, per bulan, per tahun, dan antartahun.',
    'f.recurring.t': 'Pembayaran rutin',
    'f.recurring.d': 'Masukkan sewa, langganan, atau cicilan sekali saja. Kinkit menambahkannya sesuai jadwal, dan Anda bisa menjeda yang mana saja.',
    'f.goals.t': 'Target tabungan dan kekayaan bersih',
    'f.goals.d': 'Tiap target menunjukkan berapa yang perlu ditabung per bulan agar tercapai tepat waktu. Pantau saldo menuju target kekayaan bersih.',
    'f.receipts.t': 'Struk, dibaca di HP Anda',
    'f.receipts.d': 'Simpan foto bersama transaksinya. Totalnya dibaca di perangkat Anda, jadi fotonya tidak pernah dikirim untuk dipindai.',
    'f.csv.t': 'Impor CSV bank',
    'f.csv.d': 'BCA, Mandiri, atau CSV umum. Pratinjau dulu; mengimpor file yang sama dua kali tidak membuat duplikat, dan setiap impor bisa dibatalkan.',
    'f.dash.t': 'Dasbor sesuai selera',
    'f.dash.d': 'Minggu, Bulan, Tahun, dan Antartahun, plus tab Kustom untuk memilih dan mengurutkan grafik. Ekspor ke PDF.',
    'f.fx.t': 'Tampilan multi-mata uang',
    'f.fx.d': 'Tampilkan jumlah dalam IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, atau INR dengan kurs harian. Data tersimpan tetap dalam rupiah.',

    'g.home': 'Rumah &amp; keluarga',
    'g.homeSub': 'Rencanakan, bagikan, dan jalankan rumah tangga bersama, dengan akses yang pas untuk tiap orang.',

    'f.household.t': 'Semua orang bisa masuk',
    'f.household.d': 'Tiap anggota keluarga punya email dan kata sandi sendiri, di rumah tangga yang sama.',
    'f.roles.t': 'Peran yang pas untuk keluarga nyata',
    'f.roles.d': 'Admin dan orang tua mengelola semuanya. Anak hanya mengubah entrinya sendiri. Pengamat, seperti kakek-nenek atau akuntan, bisa melihat tanpa mengubah.',
    'f.allowance.t': 'Uang saku anak',
    'f.allowance.d': 'Beri tiap anak uang saku bulanan dan biarkan mereka melihat sisanya.',
    'f.undo.t': 'Urungkan untuk penghapusan sehari-hari',
    'f.undo.d': 'Hapus transaksi, tugas, atau pengingat dan tombol Urungkan muncul beberapa detik. Hanya penghapusan besar, seperti seluruh daftar atau satu impor, yang bertanya dulu.',
    'f.today.t': 'Hari ini',
    'f.today.d': 'Satu tampilan harian untuk yang jatuh tempo: pengingat, kegiatan, tugas, daftar, usulan menu, dan anggaran.',
    'f.chores.t': 'Tugas rumah dan daftar belanja',
    'f.chores.d': 'Tugas rumah memberi poin, dengan persetujuan orang tua dan bukti foto. Ketik "telur, 2 susu, roti x3" di daftar bersama, lalu jadikan belanjaan satu catatan pengeluaran.',

    'g.fun': 'Seru &amp; memotivasi',
    'g.funSub': 'Ajari anak lewat praktik: tugas rumah, uang saku, target, poin, dan lencana.',

    'f.cal.t': 'Kalender keluarga',
    'f.cal.d': 'Les renang, latihan sepak bola, libur sekolah. Tampilan bulan, minggu, hari, dan agenda, kegiatan berulang, dan jadwal tiap orang.',
    'f.meals.t': 'Menu makan yang bisa diusulkan siapa saja',
    'f.meals.d': 'Menu mingguan yang disusun dari bahan wajib pakai dan promo toko. Anak bisa mengusulkan hidangan; orang tua menyetujuinya untuk suatu hari.',
    'f.widgets.t': 'Layar widget yang Anda atur',
    'f.widgets.d': 'Seret jam, agenda, tugas, menu, anggaran, dan lainnya ke tempatnya, lalu tampilkan layar penuh di HP, TV, atau layar mobil.',
    'f.points.t': 'Level 1 sampai 50',
    'f.points.d': '10 poin per hari untuk mencatat, plus bonus streak, dari Getting Started lewat Budget Buddy dan Money Master sampai Legend.',
    'f.badges.t': '20+ lencana perunggu, perak, dan emas',
    'f.badges.d': 'Raih Financial Geek, Goal Crusher, Streak 30, Chore champion, Chef\'s Choice, dan lainnya, masing-masing dengan perayaan kecil.',
    'f.challenges.t': 'Tantangan keluarga mingguan',
    'f.challenges.d': 'Catat setiap hari, akhir pekan tanpa belanja, jaga Keinginan di bawah jatah seminggu. Tiap tantangan menambah poin bonus ke total keluarga.',
    'f.leader.t': 'Papan peringkat keluarga',
    'f.leader.d': 'Lihat siapa yang paling banyak mengumpulkan poin bulan ini.',
    'f.themes.t': 'Tema untuk segala usia',
    'f.themes.d': 'Terang dan Gelap, tema Ceria untuk anak, dan tema Senior berkontras tinggi dengan teks lebih besar.',
    'f.themes.link': 'Lihat temanya',

    'how.title': 'Satu makan siang, dari awal sampai akhir',
    'how.lead': 'Inilah yang terjadi saat seseorang di keluarga membeli makan siang seharga Rp25.000. Tanpa membuka aplikasi, tanpa spreadsheet.',

    'screens.title': 'Di HP, laptop, dan TV ruang keluarga',
    'screens.lead': 'Keluarga yang sama dan angka yang sama, menyesuaikan di mana pun keluarga Anda berada.',
    'screens.phoneAlt': 'Kinkit di HP: pengeluaran bulan ini dibanding anggaran, pengeluaran terbaru, dan level Anda.',
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

    'privacy.title': 'Data keluarga Anda tetap milik Anda',
    'privacy.lead': 'Kinkit tidak pernah meminta login bank Anda. Anda yang menentukan apa yang masuk, dan Anda bisa mengambil semuanya kapan saja.',
    'privacy.1.t': 'Ekspor kapan saja',
    'privacy.1.d': 'Unduh pengeluaran (per rentang tanggal) dan pemasukan sebagai CSV, atau cadangan lengkap semuanya sebagai JSON.',
    'privacy.2.t': 'Peran yang menjaga ketertiban',
    'privacy.2.d': 'Anak hanya bisa mengubah entrinya sendiri, dan pengamat seperti kakek-nenek atau akuntan bisa melihat tetapi tidak bisa mengubah.',
    'privacy.3.t': 'Struk tetap privat',
    'privacy.3.d': 'Foto struk hanya bisa dilihat keluarga Anda, dan jumlahnya dibaca di perangkat Anda sendiri.',
    'privacy.4.t': 'Anda yang mengatur berbagi',
    'privacy.4.d': 'Tautan TV hanya menampilkan ringkasan bulan ini atau papan widget yang Anda pilih, dan sekali klik mencabutnya.',
    'privacy.formats': 'Format ekspor',
    'privacy.f1': 'Setiap pengeluaran, untuk rentang tanggal apa pun',
    'privacy.f2': 'Semua pemasukan yang Anda catat',
    'privacy.f3': 'Cadangan lengkap semuanya',

    'faq.title': 'Pertanyaan yang sering diajukan keluarga',
    'faq.q1': 'Apakah saya harus menghubungkan rekening bank?',
    'faq.a1': 'Tidak. Kinkit tidak pernah meminta kredensial bank. Catat pengeluaran di aplikasi atau lewat chat, atau impor CSV mutasi yang sudah bisa Anda unduh dari bank (BCA, Mandiri, atau format umum). Anda melihat pratinjau sebelum apa pun disimpan.',
    'faq.q2': 'Apakah anak-anak bisa memakainya?',
    'faq.a2': 'Bisa. Beri mereka peran Anak. Mereka bisa mencatat pengeluaran sendiri, hanya mengubah entri milik sendiri, dan melihat uang saku bulanannya. Mereka juga mendapat poin dan lencana, dan tema Ceria dibuat untuk mereka.',
    'faq.q3': 'Bisakah saya mengambil data saya?',
    'faq.a3': 'Selalu. Di Pengaturan › Data &amp; perangkat Anda bisa mengekspor pengeluaran (opsional per rentang tanggal) dan pemasukan sebagai CSV, atau mengunduh cadangan JSON lengkap.',
    'faq.q5': 'Apakah bisa dipakai tanpa internet?',
    'faq.a5': 'Aplikasi yang terpasang tetap terbuka saat offline dan menampilkan data dari kunjungan terakhir. Pengeluaran yang dicatat saat offline disimpan di perangkat dan disinkronkan otomatis saat tersambung lagi. Jika server menolak sebuah entri, entri itu ditampilkan sehingga tidak ada yang hilang diam-diam.',
    'faq.q6': 'Bagaimana cara kerja bot chat?',
    'faq.a6': 'Kinkit bekerja dengan Telegram, Discord, Slack, dan Matrix. Buat bot Anda sendiri (misalnya lewat BotFather di Telegram) lalu tempel token dan chat id-nya di Pengaturan › Bot; Matrix juga butuh alamat homeserver Anda. Setelah itu kirim pesan seperti "50k makan siang" untuk mencatat pengeluaran, atau <code>/recent</code> (<code>!recent</code> di Discord) untuk melihat entri terakhir. <code>/remind</code> membuat pengingat, <code>/buy</code> menambah daftar belanja, <code>/chores</code> menampilkan tugas, dan <code>/menu</code> rencana menu minggu ini. Bot yang sama juga mengirim peringatan anggaran.',
    'faq.q7': 'Mata uang apa saja yang didukung?',
    'faq.a7': 'Jumlah disimpan dalam rupiah (IDR). Anda bisa menampilkannya dalam IDR, USD, EUR, GBP, SGD, MYR, JPY, AUD, atau INR, dikonversi dengan kurs harian.',
    'faq.q8': 'Apakah ada aplikasi HP atau TV?',
    'faq.a8': 'Ada. Tersedia aplikasi Android untuk HP, dan aplikasi TV khusus tampilan untuk Android TV / Google TV / Fire TV dan LG webOS, plus proyek Samsung Tizen untuk TV dalam mode developer. Di iPhone, iPad, dan komputer, pasang aplikasi webnya dari browser; aplikasi iPhone native segera hadir. Browser TV atau mobil mana pun juga bisa membuka tautan tampilan TV.',

    'final.title': 'Beri seisi rumah satu tempat untuk melihat',
    'final.lead': 'Buat rumah tangga, undang semua orang, dan kirim "25k makan siang" pertama Anda. Lencana untuk catatan pertama ikut datang.',

    'footer.top': 'Kinkit, kembali ke atas',
    'footer.about': 'Kelola seluruh rumah tangga, bukan hanya anggarannya. Dibuat untuk keluarga di Indonesia dan sekitarnya.',

    'f.reminders.t': 'Pengingat yang sampai ke orang yang tepat',
    'f.reminders.d': 'Cucian, tandon air, tagihan, obat, ulang tahun. Berulang tiap hari, tiap 2 minggu, atau tiap 3 bulan, dengan colekan di chat keluarga.',

    'more.money': '6 fitur keuangan lainnya',
    'more.home': '6 lagi untuk rumah dan keluarga',

    'how.b1.where': 'di Telegram',
    'how.b1.t': 'Kirim ke bot keluarga',
    'how.b1.d': 'Bot memilih kategori dari kata kunci Anda sendiri. Kata baru muncul dengan tombol kategori, dan bot mengingat pilihannya.',
    'how.b2.where': 'di anggaran',
    'how.b2.t': 'Anggaran diperbarui untuk semua',
    'how.b2.d': 'Web dan HP langsung menampilkan angka baru, dan TV ruang keluarga menyusul dalam satu menit.',
    'how.b3.where': 'di chat keluarga',
    'how.b3.t': 'Melewati 80% mengirim peringatan',
    'how.b3.d': 'Dalam beberapa menit: sekali di 80%, dan sekali lagi jika terlampaui. Tidak perlu membuka aplikasi untuk tahu.',
    'how.b4.where': 'di papan peringkat',
    'how.b4.t': 'Mencatat memberi poin',
    'how.b4.d': '10 poin untuk mencatat hari ini, plus bonus streak. Anak menaiki level yang sama dengan orang tuanya.',
    'how.note': 'Penyiapan cukup satu kali daftar: Anda menjadi Admin Keluarga, mengundang yang lain, dan memberi tiap orang peran. Lebih suka mengetik? Entri yang sama hanya butuh beberapa detik di aplikasi, bahkan saat offline.',

    'privacy.more': 'Cara Kinkit menangani data Anda',

    'footer.nav': 'Footer',
    'footer.privacy': 'Privasi &amp; data',

    'pp.meta': 'Privasi dan data: Kinkit',
    'pp.title': 'Privasi dan data Anda',
    'pp.updated': 'Terakhir diperbarui 20 November 2026',
    'pp.lead': 'Halaman ini menjelaskan apa yang benar-benar dilakukan aplikasi Kinkit dengan data keluarga Anda saat ini. Ini penjelasan sederhana, bukan kontrak hukum.',
    'pp.store.h': 'Apa yang disimpan Kinkit',
    'pp.store.1': 'Akun Anda: nama, email, dan kata sandi. Kata sandi hanya disimpan sebagai hash satu arah, tidak pernah sebagai teks.',
    'pp.store.2': 'Apa yang dimasukkan keluarga Anda: pengeluaran, pemasukan, anggaran, target, aset, tugas rumah, pengingat, daftar, kegiatan kalender, dan rencana menu.',
    'pp.store.3': 'Foto struk yang Anda lampirkan ke transaksi.',
    'pp.store.4': 'Detail bot chat yang Anda tempel di Pengaturan › Bot (token, chat id, homeserver Matrix), agar Kinkit bisa mengirim pengingat dan peringatan.',
    'pp.not.h': 'Apa yang tidak dikumpulkan Kinkit',
    'pp.not.1': 'Login bank. Kinkit tidak pernah memintanya; impor memakai file CSV yang sudah bisa Anda unduh dari bank.',
    'pp.not.2': 'Data iklan atau pelacakan. Aplikasi dan situs ini tidak memuat skrip analitik, iklan, atau pelacak.',
    'pp.where.h': 'Di mana data disimpan dan siapa yang bisa melihatnya',
    'pp.where.1': 'Semuanya disimpan di server yang menjalankan aplikasi Kinkit Anda: database untuk entri dan folder privat untuk foto struk. Anggota keluarga melihatnya sesuai peran: anak hanya mengubah entrinya sendiri, dan pengamat hanya bisa melihat. Pengelola server dapat mengatur akun.',
    'pp.where.2': 'Tautan tampilan TV hanya menampilkan ringkasan bulan ini atau papan widget yang Anda pilih, tidak butuh login, dan langsung berhenti bekerja begitu Anda mencabutnya.',
    'pp.third.h': 'Layanan yang dihubungi Kinkit',
    'pp.third.1': 'Pembacaan struk berjalan di perangkat Anda. Saat Anda mengetuk "Baca jumlah", pembacanya (tesseract.js) diunduh dari CDN jsDelivr; fotonya sendiri tidak dikirim ke mana pun untuk dibaca.',
    'pp.third.2': 'Kurs berasal dari open.er-api.com. Kinkit hanya meminta kurs hari itu; tidak ada data Anda yang dikirim.',
    'pp.third.3': 'Jika Anda menghubungkan bot chat, pesan yang dikirim dan diterima melewati Telegram, Discord, Slack, atau server Matrix Anda, dengan ketentuan privasi masing-masing.',
    'pp.cookies.h': 'Cookie dan penyimpanan lokal',
    'pp.cookies.1': 'Aplikasi memasang satu cookie sesi agar Anda tetap masuk. Aplikasi yang terpasang menyimpan salinan data terbaru dan pengeluaran yang dicatat saat offline di perangkat sampai tersinkron. Situs ini mengingat pilihan bahasa Anda di browser.',
    'pp.out.h': 'Mengambil data Anda',
    'pp.out.1': 'Di Pengaturan › Data &amp; perangkat Anda bisa mengunduh pengeluaran dan pemasukan sebagai CSV, atau cadangan lengkap sebagai JSON, kapan saja.',
    'pp.delete.h': 'Menghapus data Anda',
    'pp.delete.1': 'Anda bisa menghapus entri, daftar, struk, dan tautan TV sendiri. Belum ada tombol untuk menghapus seluruh akun sendiri; pengelola server bisa menghapusnya atas permintaan.',
    'pp.contact': 'Alamat kontak untuk pertanyaan privasi dan permintaan penghapusan akan ditambahkan di sini sebelum peluncuran publik.',
    'pp.back': 'Kembali ke Kinkit'
  };

  var LANG_KEY = 'kinkit_landing_lang';
  var titleKey = root.getAttribute('data-i18n-title') || 'meta.title'; // each page names its own title key
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
    EN[titleKey] = document.title;
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
    document.title = (dict[titleKey] || EN[titleKey]);
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
