export interface Stream {
  id: string;
  host: string;
  members: string[];
}

export class SyncStreams {
  public streams: Stream[];

  constructor() {
    this.streams = [];
  }

  public addStream(stream: Stream): void {
    this.streams.push(stream);
  }

  public removeStream(id: string): void {
    this.streams = this.streams.filter(stream => stream.id !== id);
  }

  public getStream(id: string): Stream | undefined {
    return this.streams.find(stream => stream.id === id);
  }

  public addMember(streamId: string, member: string): void {
    const stream = this.getStream(streamId);
    if (stream) {
      stream.members.push(member);
    }
  }
}
