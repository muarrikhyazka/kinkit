import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/capture/notification_parser.dart';

void main() {
  ParsedCapture? parse(String text, {String title = '', String app = ''}) =>
      parseCapture(title: title, text: text, app: app);

  group('amounts', () {
    test('Rp 25.000 reads as 25000', () {
      expect(parse('Payment Rp 25.000')!.amount, 25000);
    });

    test('Rp25.000,00 drops the decimals', () {
      final r = parse('Paid Rp25.000,00 today')!;
      expect(r.amount, 25000);
      expect(r.currency, 'IDR');
    });

    test('IDR 1,250,000.00 reads as 1250000', () {
      expect(parse('IDR 1,250,000.00 charged')!.amount, 1250000);
    });

    test('Rp 1.250.000 reads as 1250000', () {
      expect(parse('Debit Rp 1.250.000')!.amount, 1250000);
    });

    test(r'$12.50 rounds to 13 USD', () {
      final r = parse(r'Card charged $12.50')!;
      expect(r.amount, 13);
      expect(r.currency, 'USD');
    });

    test('euro and pound symbols map to their codes', () {
      expect(parse('Paid €9,99 online')!.currency, 'EUR');
      expect(parse('Paid £4.20 online')!.currency, 'GBP');
    });

    test('trailing sentence period is not part of the number', () {
      expect(parse('You spent Rp 48.000.')!.amount, 48000);
    });

    test('the first amount wins over a later balance', () {
      expect(parse('Paid Rp 54.000. Balance Rp 1.000.000')!.amount, 54000);
    });

    test('number then IDR suffix', () {
      expect(parse('Transfer of 150.000 IDR done')!.amount, 150000);
    });

    test('SGD, MYR, USD, EUR and GBP codes keep their own currency', () {
      final sgd = parse('Card payment SGD 12.50 at Cold Storage')!;
      expect(sgd.currency, 'SGD');
      expect(sgd.amount, 13);
      expect(parse('Paid MYR 30.00 at Giant')!.currency, 'MYR');
      expect(parse('Paid USD 5 online')!.currency, 'USD');
      expect(parse('Paid EUR9,99 online')!.currency, 'EUR');
      expect(parse('Paid GBP 4.20 online')!.currency, 'GBP');
      expect(parse('Paid 12.50 SGD at Shell')!.currency, 'SGD');
    });

    test('juta, jt, ribu, rb and k after a number scale it', () {
      expect(parse('Pembayaran Rp1.5 juta berhasil')!.amount, 1500000);
      expect(parse('Debit Rp 2,5 jt di Tokopedia')!.amount, 2500000);
      expect(parse('Debit Rp 50 rb di Warung')!.amount, 50000);
      expect(parse('Paid Rp 75k at Kopi')!.amount, 75000);
    });

    test('a word that only starts with k is not a magnitude', () {
      expect(parse('Debit Rp 5.000 kode ref 1')!.amount, 5000);
    });

    test('a zero amount is skipped for the next readable one', () {
      expect(parse('Rp 0 hold, Rp 5.000 charged')!.amount, 5000);
    });
  });

  group('money marker', () {
    test('matches the same currencies the Kotlin filter queues', () {
      for (final t in ['Rp 5', 'IDR5', r'$5', '€5', '£5', 'SGD 5', 'MYR 5', 'USD 5', 'EUR 5', 'GBP 5', '150 IDR', '5 rupiah']) {
        expect(moneyMarker.hasMatch(t), isTrue, reason: t);
      }
      expect(moneyMarker.hasMatch('Sharp 5 items'), isFalse);
    });
  });

  group('parseLocaleNumber', () {
    test('both separators: the last one is decimal', () {
      expect(parseLocaleNumber('1.250.000,50'), 1250000.5);
      expect(parseLocaleNumber('1,250,000.50'), 1250000.5);
    });

    test('a repeated separator is thousands', () {
      expect(parseLocaleNumber('1.250.000'), 1250000);
      expect(parseLocaleNumber('1,250,000'), 1250000);
    });

    test('one separator then three digits is thousands, otherwise decimal', () {
      expect(parseLocaleNumber('25.000'), 25000);
      expect(parseLocaleNumber('12.50'), 12.5);
      expect(parseLocaleNumber('12,5'), 12.5);
    });

    test('not a number returns null', () {
      expect(parseLocaleNumber('abc'), isNull);
      expect(parseLocaleNumber(''), isNull);
    });
  });

  group('merchant', () {
    test('"to X successful" stops before the status word', () {
      final r = parse('Payment of Rp 54.000 to NETFLIX successful')!;
      expect(r.merchant, 'NETFLIX');
      expect(r.isIncome, isFalse);
    });

    test('"at X" runs to the end of the text', () {
      expect(parse('You paid Rp 48.000 at Super Indo')!.merchant, 'Super Indo');
    });

    test('stops at a comma, a period or " on"', () {
      expect(parse('Rp 20.000 at Warung Bu Tini, thanks')!.merchant, 'Warung Bu Tini');
      expect(parse('Rp 20.000 at Indomaret on 24/11')!.merchant, 'Indomaret');
    });

    test('falls back to a specific title', () {
      final r = parse('Pembayaran Rp 54.000 berhasil', title: 'GoFood')!;
      expect(r.merchant, 'GoFood');
    });

    test('a generic title is skipped in favour of the app name', () {
      final r = parse('Rp 54.000 berhasil', title: 'Transaksi berhasil', app: 'Bank Demo')!;
      expect(r.merchant, 'Bank Demo');
    });

    test('a package name is never used as the merchant', () {
      final r = parse('Rp 54.000 berhasil', title: 'Info', app: 'com.bank.demo')!;
      expect(r.merchant, '');
    });

    test('merchant is capped at 40 characters', () {
      final r = parse('Rp 10.000 at ${'A' * 80}')!;
      expect(r.merchant.length <= 40, isTrue);
    });
  });

  group('money in', () {
    test('transfer received is income', () {
      expect(parse('You received Rp 500.000 from Budi')!.isIncome, isTrue);
    });

    test('Indonesian incoming transfer is income', () {
      expect(parse('Dana masuk Rp 250.000')!.isIncome, isTrue);
    });

    test('refund is income', () {
      expect(parse('Refund of Rp 35.000 processed')!.isIncome, isTrue);
    });

    test('a normal payment is not income', () {
      expect(parse('Payment Rp 35.000 to Grab')!.isIncome, isFalse);
    });

    test('"termasuk" contains "masuk" but is not income', () {
      expect(parse('Pembelian Rp 100.000 di Toko, termasuk pajak')!.isIncome, isFalse);
      expect(parse('Total Rp 100.000 termasuk ongkir')!.isIncome, isFalse);
    });

    test('"diterima" or "received" on a payment confirmation is an expense', () {
      expect(parse('Pembayaran Rp 54.000 ke NETFLIX diterima')!.isIncome, isFalse);
      expect(parse('Your payment of Rp 54.000 to Grab was received')!.isIncome, isFalse);
    });

    test('"diterima" alone is income', () {
      expect(parse('Dana diterima Rp 250.000 dari Budi')!.isIncome, isTrue);
    });
  });

  group('rejected', () {
    test('one-time code returns null', () {
      expect(parse('Your OTP is 482913. Do not share. Purchase Rp 50.000 at Shop'), isNull);
    });

    test('verification code returns null', () {
      expect(parse('Verification code 1234 for payment Rp 10.000'), isNull);
    });

    test('promotion returns null', () {
      expect(parse('Promo! Get Rp 10.000 off your next order'), isNull);
    });

    test('voucher returns null', () {
      expect(parse('Your voucher worth Rp 25.000 expires soon'), isNull);
    });

    test('"kode ref" with a debit is a payment, not a code', () {
      final r = parse('Kode ref 123456 Debit Rp 50.000 di ALFAMART')!;
      expect(r.amount, 50000);
      expect(r.isIncome, isFalse);
      expect(r.merchant, 'ALFAMART');
    });

    test('kode OTP / verifikasi / verification and one-time are codes', () {
      expect(parse('Kode OTP 123456 untuk bayar Rp 10.000'), isNull);
      expect(parse('Kode verifikasi 5521, Rp 10.000'), isNull);
      expect(parse('Kode verification 5521, Rp 10.000'), isNull);
      expect(parse('One-time password 5521 for Rp 10.000'), isNull);
      expect(parse('Your one time PIN is 9911, Rp 10.000'), isNull);
    });

    test('a payment that merely mentions a promo still parses', () {
      final r = parse('Pembayaran Rp 54.000 ke GoFood berhasil. Promo cashback menanti!')!;
      expect(r.amount, 54000);
      expect(parse('Rp 25.000 at Indomaret. Check the promo!')!.merchant, 'Indomaret');
    });

    test('isCaptureNoise is true only for codes and bare promotions', () {
      expect(isCaptureNoise(title: '', text: 'OTP 1234'), isTrue);
      expect(isCaptureNoise(title: '', text: 'Promo Rp 10.000 off'), isTrue);
      expect(isCaptureNoise(title: '', text: 'Payment Rp 10.000 promo applied'), isFalse);
      expect(isCaptureNoise(title: '', text: 'Rp 0 hold'), isFalse);
    });

    test('no amount returns null', () {
      expect(parse('Your payment was successful'), isNull);
    });

    test('a zero amount returns null', () {
      expect(parse('Rp 0 charged'), isNull);
    });

    test('words that merely contain "rp" are not currency', () {
      expect(parse('Sharp 5 items shipped'), isNull);
    });
  });

  group('draftForReview', () {
    test('a readable payment is the normal parse', () {
      final d = draftForReview(title: '', text: 'Payment Rp 54.000 to NETFLIX successful')!;
      expect(d.amountKnown, isTrue);
      expect(d.amount, 54000);
    });

    test('a currency marker with no readable amount is kept with amount unknown', () {
      final d = draftForReview(title: 'Bank Demo', text: 'Debit Rp 0 at Kopi Kenangan')!;
      expect(d.amountKnown, isFalse);
      expect(d.merchant, 'Kopi Kenangan');
    });

    test('codes, bare promotions and text with no money are deleted', () {
      expect(draftForReview(title: '', text: 'OTP 482913 for Rp 10.000'), isNull);
      expect(draftForReview(title: '', text: 'Promo! Get Rp 10.000 off your next order'), isNull);
      expect(draftForReview(title: '', text: 'Your payment was successful'), isNull);
    });
  });

  group('captureClientId', () {
    test('is stable and prefixed', () {
      final a = captureClientId('0|com.bank|1|null|10001|1700000000000');
      expect(a, captureClientId('0|com.bank|1|null|10001|1700000000000'));
      expect(a.startsWith('notif-'), isTrue);
      expect(a.length, 'notif-'.length + 16);
    });

    test('differs between notifications', () {
      expect(captureClientId('a|1'), isNot(captureClientId('a|2')));
    });
  });
}
