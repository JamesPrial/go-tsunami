# go-tsunami


Brief Introduction to the Tsunami UDP Protocol

The Tsunami protocol was created in 2002 by the Pervasive Technology Labs of Indiana University and was
released to the public as open-source under a UI Open Source License. After release, several people have
improved the code and have added more functionality. Currently two branched versions of the Tsunami UDP
protocol, generic and real-time, are maintained by Metsähovi Radio Observatory / Jan Wagner. Everyone
interested is invited to join development at http://tsunami-udp.sf.net.

Tsunami is one of several currently available UDP-based transfer protocols that were developed for high
speed transfer over network paths that have a high bandwidth-delay product. Such paths can for example
be found in the European GEANT research network. It can for example be a route from a local server PC
to a local GEANT2 access node such as FUNET or SURFNET, then via GEANT's internal 10G to another country,
and finally a local link via another node such as NORDUNET to some client PC. Currently these network
links are capable of 1G..10G and can have some hundred milliseconds of roundtrip delay between the client
and server PCs.

Custom UDP protocols are needed because average TCP/IP is not very well suited for paths with a large 
bandwidth-delay product. To take full advantage of the available bandwidth, the standard TCP slow-start 
congestion control algorithm needs to be replaced with e.g. HighSpeed TCP, Compound TCP, or one of several 
others. Some TCP parameters need to be tweaked, such as SACK, dynamic TCP window size. Most of the extended 
TCP features are already part of Windows Vista, with some partly implemented but not enabled in Windows XP
and 2000. In Linux, far more extensive support and a much broader choice of options and congestion
control algorithms is available.

Currently Tsunami runs in Linux and Unix. The source code should be largely POSIX compliant. It might be
easily ported to Windows XP and Vista as they have a POSIX layer, but, porting has not been attempted yet.

In Very Long Baseline Interferometry, an interferometry based observation method in radio astronomy, the
digitized noise recorded from stellar radio sources is often streamed over the network, and there's no
requirement for reliable data transport as is guaranteed with TCP. A fraction of data can be lost without
degrading the sensitivity of the method too much. In some cases, for example to maintain a high data stream 
throughput, it may be be preferred to just stream the data and not care about transmission errors i.e. not 
request retransmission of old missing data.

The Tsunami UDP protocol has several advantages over TCP and most other UDP-based similar protocols: it is
high-speed (a maintained 900Mbps through 1Gbit NICs and switches isn't unusual), it offers data transmission
with default priority for data integrity, but may also be switched to rate priority by disabling
retransmissions. It is the most stable of the other available UDP-based similar protocols.
Tsunami is completely implemented in user land and thus doesn't depend on Linux kernel extensions. It can be
compiled and run from a normal user account, even a guest account, and does not need root privileges. The
global settings for a file transfer are specified by the client - this is useful since the Tsunami user
often has a priori knowledge about the quality of their network connection and the speed of their harddisks,
and can pass the suitable settings through the client application to the server. Another benefit of Tsunami is
that the client already contains a command line interface, no external software is necessary. The commands
available in the Tsunami client are similar to FTP commands.


How It Works

Tsunami performs a file transfer by sectioning the file into numbered blocks of usually 32kB size.
Communication between the client and server applications flows over a low bandwidth TCP connection.
The bulk data is transferred over UDP.

Most of the protocol intelligence is worked into the client code - the server simply sends out all blocks,
and resends blocks that the client requests. The client specifies nearly all parameters of the transfer, 
such as the requested file name, target data rate, blocksize, target port, congestion behaviour, etc, and 
controls which blocks are requested from the server and when these requests are sent.

Clients connect to the server and authenticate with a MD5 hash in challenge-response using a shared secret.
The default is "kitten". In the current client, per default the user is not asked for a password.

The client starts file transfers with a get-file request. At the first stage of a transfer the client passes 
all its transfer parameters to the server inside the request. The server reports back the length of the
requested file in bytes, so that the client can calculate how many blocks it needs to receive. 

Immediately after a get-file request the server begins to send out file blocks on its own, starting from the 
first block. It flags these blocks as "original blocks". The client can request blocks to be sent again. These
blocks are flagged as "retransmitted blocks" by the server.

When sending out blocks, to throttle the transmission rate to the rate specified by the client, the server pauses 
for the correct amount of time after each block before sending the next.

The client regularly sends error rate information to the server. The server uses this information to adjust
the transmission rate; the server can gradually slow down the transmission when the client reports it is
too slow in receiving and processing the UDP packets. This, too, is controlled by the cient. In the settings
passed from client to server at the start of a transfer, the client configures the server's speed of slowdown
and recovery/"speed-up", and specifies an acceptable packet loss percentage (for example 7%).

The client keeps track of which of the numbered blocks it has already received and which blocks are still
pending. This is done by noting down the received blocks into a simple bitfield. When a block has been
received, in the bitfield the bit corresponding to the received block is set to '1'.

If the block number of a block that the client receives is larger than what would be the correct and expected
consecutive block, the missing intervening blocks are queued up for a pending retransmission. The
retransmission "queue" is a simple sorted list of the missing block numbers. The list size is allowed to
grow dynamically, to a limit. At regular intervals, the retransmission list is processed - blocks that
have been received in the meantime are removed from the list, after which the list of really missing blocks
is sent as a normal block transmission request to the server.

When adding a new pending retransmission to the client's list makes the list exceed a hard-coded limit, the
entire transfer is reinitiated to start at the first block in the list i.e. the earliest block in the entire
file that has not been successfully transferred yet. This is done by sending a special restart-transmission
request to the server.

When all blocks of the file have been successfully received, the client sends a terminate-transmission request
to the server.

During a file transfer, both server and client applications regularly output a summary of transfer statistics
to the console window, reporting the target and actual rate, transmission error percentage, etc. These
statistics may be used in e.g. Matlab to graph the characteristics of the transfer and network path.

All client file I/O is performed in a separate disk thread, with a memory ring buffer used to communicate
new data from the main process to the I/O thread for writing to disk.

In pseudo-code, the server and client operate approximately like this:

**Server**
 start
 while(running) {
   wait(new incoming client TCP connection)
   fork server process:
   [
     check_authenticate(MD5, "kitten");
     exchange settings and values with client;
     while(live) {
       wait(request, nonblocking)
       switch(request) {
          case no request received yet: { send next block in sequence; }
          case request_stop:            { close file, clean up; exit; }
          case request_retransmit:      { send requested blocks; }
       }
       sleep(throttling)
     }
   ]
 }

**Client**
 start, show command line
 while(running) {
    read user command;
    switch(command) {
       case command_exit:    { clean up; exit; }
       case command_set:     { edit the specified parameter; }
       case command_connect: { TCP connect to server; auth; protocol version compare;
                               send some parameters; }
       case command_get && connected:  { 
           send get-file request containing all transfer parameters;
           read server response - filesize, block count;
           initialize bit array of received blocks, allocate retransmit list;
           start separate disk I/O thread;
           while (not received all blocks yet) {
              receive_UDP();
              if timeout { send retransmit request(); }

              if block not marked as received yet in the bit array {
                 pass block to I/O thread for later writing to disk;
                 if block nr > expected block { add intermediate blocks to retransmit list; }
              }

              if it is time { 
                 process retransmit list, send assembled request_retransmit to server;
                 send updated statistics to server, print to screen;
              } 
           }
           send request_stop;
           sync with disk I/O, finalize, clean up;
       }
       case command_help:    { display available commands etc; }
    }
 }

taken from https://sourceforge.net/p/tsunami-udp/code_git/ci/master/tree/docs/howTsunamiWorks.txt#l1