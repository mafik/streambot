#include <thread>
#include <cstdio>

#include "tobii_gameintegration.h"

using namespace TobiiGameIntegration;
using namespace std::literals::chrono_literals;

int main() {
  ITobiiGameIntegrationApi* api = GetApi("Gaze Sample");
  IStreamsProvider* streamsProvider = api->GetStreamsProvider();

  api->GetTrackerController()->TrackRectangle({0,0,1920,1080});

  while(true) {
    api->Update();

    GazePoint gazePoint;
    if (streamsProvider->GetLatestGazePoint(gazePoint)) {
      printf("%f %f\n", gazePoint.X, gazePoint.Y);
      fflush(stdout);
    }

    std::this_thread::sleep_for(1s / 60);
  }

  api->Shutdown();
}