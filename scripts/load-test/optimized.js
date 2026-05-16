import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// =========================
// Custom Metrics
// =========================
export const successRate        = new Rate('success_rate');
export const failedRequests     = new Counter('failed_requests');
export const writeLatency       = new Trend('write_latency');
export const readBalanceLatency = new Trend('read_balance_latency');
export const readStatusLatency  = new Trend('read_status_latency');

// =========================
// Baseline Test Config
// Target: ~500.000 requests (70% read, 30% write)
// =========================
export const options = {
  scenarios: {
    baseline_load: {
      executor: 'ramping-arrival-rate',
      startRate: 20,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 300,
      stages: [
        { target: 20,  duration: '10s' },
        { target: 50,  duration: '15s' },
        { target: 100, duration: '30s' },
        { target: 150, duration: '45s' },
        { target: 150, duration: '30s' },
        { target: 50,  duration: '15s' },
        { target: 0,   duration: '5s' },
      ],
    },
  },

  thresholds: {
    http_req_failed:      ['rate<0.25'],
    http_req_duration:    ['p(95)<10000'],
    write_latency:        ['p(95)<10000'],
    read_balance_latency: ['p(95)<5000'],
  },
};

// =========================
// Helpers
// =========================
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Akun yang ada di DB (1001–2000, subset dari 100K yang di-seed)
const ACCOUNTS = Array.from({ length: 1000 }, (_, i) => 1001 + i);

function randomAccountId() {
  return ACCOUNTS[Math.floor(Math.random() * ACCOUNTS.length)];
}

function randomAmount() {
  return Math.floor(Math.random() * 100000) + 10000;
}

// Hot accounts: 10 akun yang sering diakses (simulasi akun populer)
const HOT_ACCOUNTS = Array.from({ length: 10 }, (_, i) => 1001 + i);

function hotOrRandomAccount() {
  return HOT_ACCOUNTS[Math.floor(Math.random() * HOT_ACCOUNTS.length)];
}

const createdTxIds = [];

// =========================
// Main Test — 70% read, 30% write
// =========================
export default function () {
  const roll = Math.random();

  if (roll < 0.30) {
    group('write_transaction', () => {
      let source = randomAccountId();
      let dest   = randomAccountId();
      while (dest === source) dest = randomAccountId();

      const payload = JSON.stringify({
        source_account: source,
        dest_account:   dest,
        amount:         randomAmount(),
      });

      const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, {
        headers: { 'Content-Type': 'application/json' },
        timeout: '10s',
      });

      const ok = check(res, {
        'write: status 201': (r) => r.status === 201,
      });

      writeLatency.add(res.timings.duration);
      successRate.add(ok);

      if (!ok) {
        failedRequests.add(1);
        console.error(`WRITE FAILED | status=${res.status} body=${res.body}`);
      } else {
        try {
          const body = JSON.parse(res.body);
          if (body.id) createdTxIds.push(body.id);
        } catch (_) {}
      }
    });

  } else if (roll < 0.55) {
    group('read_balance', () => {
      const accountId = hotOrRandomAccount();

      const res = http.get(`${BASE_URL}/api/v1/accounts/${accountId}/balance`, {
        timeout: '5s',
      });

      const ok = check(res, {
        'balance: status 200': (r) => r.status === 200,
        'balance: has balance field': (r) => {
          try { return JSON.parse(r.body).balance !== undefined; } catch (_) { return false; }
        },
      });

      readBalanceLatency.add(res.timings.duration);
      successRate.add(ok);

      if (!ok) {
        failedRequests.add(1);
        console.error(`BALANCE FAILED | status=${res.status} body=${res.body}`);
      }
    });

  } else {
    group('read_tx_status', () => {
      const txId = createdTxIds.length > 0
        ? createdTxIds[Math.floor(Math.random() * createdTxIds.length)]
        : `01JNXXX${Math.floor(Math.random() * 999999)}`;

      const res = http.get(`${BASE_URL}/api/v1/transactions/${txId}/status`, {
        timeout: '5s',
      });

      const ok = check(res, {
        'status: 200 or 404': (r) => r.status === 200 || r.status === 404,
      });

      readStatusLatency.add(res.timings.duration);
      successRate.add(ok);

      if (!ok) {
        failedRequests.add(1);
        console.error(`STATUS FAILED | status=${res.status} body=${res.body}`);
      }
    });
  }

  sleep(Math.random() * 0.1);
}

// =========================
// Summary Report
// =========================
export function handleSummary(data) {
  const p95Write   = data.metrics.write_latency?.values?.['p(95)'] || 0;
  const p95Balance = data.metrics.read_balance_latency?.values?.['p(95)'] || 0;
  const errorRate  = (data.metrics.http_req_failed?.values?.rate || 0) * 100;
  const totalReqs  = data.metrics.http_reqs?.values?.count || 0;
  const rps        = data.metrics.http_reqs?.values?.rate || 0;

  const p95WriteOk   = p95Write   < 5000 ? '✅' : '❌';
  const p95BalanceOk = p95Balance < 2000 ? '✅' : '❌';
  const errorOk      = errorRate  < 25   ? '✅' : '❌';

  console.log('\n========================================');
  console.log('         BASELINE TEST SUMMARY');
  console.log('========================================');
  console.log(`Total Requests  : ${totalReqs}`);
  console.log(`Avg RPS         : ${rps.toFixed(1)} req/s`);
  console.log(`Error Rate      : ${errorRate.toFixed(2)}%  ${errorOk}`);
  console.log(`P95 Write       : ${p95Write.toFixed(0)}ms  ${p95WriteOk}`);
  console.log(`P95 Balance     : ${p95Balance.toFixed(0)}ms ${p95BalanceOk}`);
  console.log('----------------------------------------');
  console.log('Expected: High latency & errors (baseline)');
  console.log('========================================\n');

  return {
    stdout: JSON.stringify(data, null, 2),
  };
}
