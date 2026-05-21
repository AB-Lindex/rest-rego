# Run 1
**With restrego v1.0.0**

```
./e2e-tests/memleak.sh \
  http://127.0.0.1:18203 \
  http://127.0.0.1:18201 \
  e2e-tests/noauth/k6/noauth-allow.js --size=10485760
baseline heap: 3.1 MB
response size: 10485760 bytes

         /\      Grafana   /‾‾/
    /\  /  \     |\  __   /  /
   /  \/    \    | |/ /  /   ‾‾\
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/


     execution: local
        script: e2e-tests/noauth/k6/noauth-allow.js
        output: -

     scenarios: (100.00%) 1 scenario, 100 max VUs, 7m30s max duration (incl. graceful stop):
              * sustained: Up to 100 looping VUs for 7m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS

    http_req_failed
    ✓ 'rate<0.01' rate=0.00%


  █ TOTAL RESULTS

    checks_total.......: 143220  340.853014/s
    checks_succeeded...: 100.00% 143220 out of 143220
    checks_failed......: 0.00%   0 out of 143220

    ✓ status 200

    HTTP
    http_req_duration..............: avg=250.05ms min=8.48ms med=228.96ms max=1.39s p(90)=446.45ms p(95)=522.81ms
      { expected_response:true }...: avg=250.05ms min=8.48ms med=228.96ms max=1.39s p(90)=446.45ms p(95)=522.81ms
    http_req_failed................: 0.00%  0 out of 143220
    http_reqs......................: 143220 340.853014/s

    EXECUTION
    iteration_duration.............: avg=251.27ms min=8.58ms med=230.06ms max=1.39s p(90)=448.34ms p(95)=524.74ms
    iterations.....................: 143220 340.853014/s
    vus............................: 100    min=1           max=100
    vus_max........................: 100    min=100         max=100

    NETWORK
    data_received..................: 1.5 TB 3.6 GB/s
    data_sent......................: 15 MB  34 kB/s




running (7m00.2s), 000/100 VUs, 143220 complete and 0 interrupted iterations
sustained ✓ [======================================] 000/100 VUs  7m0s
waiting 30s for GC to settle...
final heap: 13.6 MB
heap delta: 10.5 MB
```

# Run 2
**With restrego patches for possible memory leak in bufferedresponse package**

```
./e2e-tests/memleak.sh \
  http://127.0.0.1:18203 \
  http://127.0.0.1:18201 \
  e2e-tests/noauth/k6/noauth-allow.js --size=10485760
baseline heap: 2.8 MB
response size: 10485760 bytes

         /\      Grafana   /‾‾/
    /\  /  \     |\  __   /  /
   /  \/    \    | |/ /  /   ‾‾\
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/


     execution: local
        script: e2e-tests/noauth/k6/noauth-allow.js
        output: -

     scenarios: (100.00%) 1 scenario, 100 max VUs, 7m30s max duration (incl. graceful stop):
              * sustained: Up to 100 looping VUs for 7m0s over 2 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ THRESHOLDS

    http_req_failed
    ✓ 'rate<0.01' rate=0.00%


  █ TOTAL RESULTS

    checks_total.......: 148660  353.826749/s
    checks_succeeded...: 100.00% 148660 out of 148660
    checks_failed......: 0.00%   0 out of 148660

    ✓ status 200

    HTTP
    http_req_duration..............: avg=240.87ms min=8.71ms med=221.49ms max=1.22s p(90)=425.55ms p(95)=494.12ms
      { expected_response:true }...: avg=240.87ms min=8.71ms med=221.49ms max=1.22s p(90)=425.55ms p(95)=494.12ms
    http_req_failed................: 0.00%  0 out of 148660
    http_reqs......................: 148660 353.826749/s

    EXECUTION
    iteration_duration.............: avg=242.06ms min=8.87ms med=222.57ms max=1.22s p(90)=427.3ms  p(95)=496.2ms
    iterations.....................: 148660 353.826749/s
    vus............................: 100    min=1           max=100
    vus_max........................: 100    min=100         max=100

    NETWORK
    data_received..................: 1.6 TB 3.7 GB/s
    data_sent......................: 15 MB  36 kB/s




running (7m00.1s), 000/100 VUs, 148660 complete and 0 interrupted iterations
sustained ✓ [======================================] 000/100 VUs  7m0s
waiting 30s for GC to settle...
final heap: 11.6 MB
heap delta: 8.8 MB
```